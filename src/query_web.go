package main

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/html"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	UrlJobs               []UrlJob `json:"url_jobs"`
	EraseStorageOnStartup bool     `json:"erase_storage_on_startup"`
	SetupServer 		bool 	`json:"server"`
	Port                  int      `json:"port"`
}

type UrlJob struct {
	Url                string
	QueryPeriodSeconds int `json:"query_period_seconds"`
	RetryPeriodSeconds int `json:"retry_period_seconds"`
}

type NameGenerator struct {
	Counter int
	Lock    sync.Mutex
}

func (n *NameGenerator) Get() string {
	n.Lock.Lock()
	next := fmt.Sprintf("%v", n.Counter)
	n.Counter += 1
	n.Lock.Unlock()

	return next
}

var nameGenerator = NameGenerator{}

func initialize(connectionString string) (Config, DbContext, error) {
	data, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	validJobs := make([]UrlJob, 0, 12)
	for i, urlJob := range config.UrlJobs {
		valid := true
		if urlJob.Url == "" {
			fmt.Printf("URL job %v has an empty or missing URL and will ignored.\n", i)
			valid = false
		}
		if urlJob.QueryPeriodSeconds <= 0 {
			fmt.Printf("URL job %v has invalid or absent query_period_seconds and will be ignored (minimum value is 1 second)\n", i)
			valid = false
		}
		if urlJob.RetryPeriodSeconds <= 0 {
			fmt.Printf("URL job %v has invalid or absent retry_period_seconds and will be ignored (minimum value is 1 second)\n", i)
			valid = false
		}

		if valid {
			validJobs = append(validJobs, urlJob)
		}
	}

	config.UrlJobs = validJobs

	exists, err := fileExists("storage.db")
	if err != nil {
		panic(err)
	}

	if config.EraseStorageOnStartup && exists {
		err := os.Remove("storage.db")
		if err != nil {
			panic(err)
		}
	}

	db, err := sql.Open("sqlite3", connectionString)
	defer db.Close()
	if err != nil {
		panic(err)
	}

	runRawQuery := func(db *sql.DB, query string) {
		stmt, err := db.Prepare(query)
		if err != nil {
			panic(err)
		}

		_, err = stmt.Exec()
		if err != nil {
			panic(err)
		}
	}

	web_pages := `CREATE TABLE IF NOT EXISTS web_pages (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT, 
		url TEXT NOT NULL);`
	runRawQuery(db, web_pages)

	web_pages_data := `CREATE TABLE IF NOT EXISTS web_pages_data (
		page_id INTEGER NOT NULL,
		data TEXT NOT NULL,
		timestamp INTEGER NOT NULL);`
	runRawQuery(db, web_pages_data)

	media := `CREATE TABLE IF NOT EXISTS media (
		id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		hash TEXT NOT NULL,
		data TEXT NOT NULL);`
	runRawQuery(db, media)

	media_web_pages := `CREATE TABLE IF NOT EXISTS media_web_pages (
		media_id INTEGER NOT NULL,
		web_page_id INTEGER NOT NULL);`
	runRawQuery(db, media_web_pages)

	var dbContext DbContext
	dbContext.ConnectionString = connectionString

	return config, dbContext, nil
}

func queryAndStoreRoutine(urlJobId int, urlJob UrlJob, dbContext *DbContext) {
	log.Printf("[Job ID: %v | URL: %v]: Start", urlJobId, urlJob.Url)
	for {
		log.Printf("[Job ID: %v | URL: %v]: Performing request..", urlJobId, urlJob.Url)
		err := queryAndSaveWebUrl(urlJob.Url, dbContext)
		if err != nil {
			log.Printf("[Job ID: %v | URL: %v]: Requesting URL failed: %v", urlJobId, urlJob.Url, err)
			time.Sleep(time.Duration(urlJob.RetryPeriodSeconds) * time.Second)
		} else {
			log.Printf("[Job ID: %v | URL: %v]: Requested parsed and saved succesfully", urlJobId, urlJob.Url)
			time.Sleep(time.Duration(urlJob.QueryPeriodSeconds) * time.Second)
		}
	}
}

type DbContext struct {
	ConnectionString string
	Mut              sync.Mutex
}

func insertMedia(data []byte, db *sql.DB) (int, string, error) {
	hash := sha256.Sum256(data)
	hash_ := make([]byte, 0, len(hash))
	for _, v := range hash {
		hash_ = append(hash_, v)
	}

	query := `select id, name from media where hash = ?`
	row := db.QueryRow(query, hash_)

	var mediaId int
	var mediaFilename string
	err := row.Scan(&mediaId, &mediaFilename)
	if err != nil {
		if err == sql.ErrNoRows {
			mediaFilename = nameGenerator.Get()
			query := `insert into media (name, hash, data) values (?, ?, ?)`
			_, err := db.Exec(query, mediaFilename, hash_, data)
			if err != nil {
				return 0, "", err
			}

			mediaId, err := getLastAutoincrementIndex(db)
			if err != nil {
				panic(err)
			}

			return mediaId, mediaFilename, nil
		} else {
			return 0, "", err
		}
	} else {
		return mediaId, mediaFilename, nil
	}
}

type MediaPage struct {
	MediaId int
	PageId  int
}

type HtmlParsingContext struct {
	RootUrl        string
	PageId         int
	Db             *sql.DB
	AlreadyRelated map[MediaPage]bool
}

func processMediaLink(ctx *HtmlParsingContext, node *html.Node, attr_i int) error {
	attr := &node.Attr[attr_i]

	var mediaUrl string
	if strings.HasPrefix(attr.Val, "http://") || strings.HasPrefix(attr.Val, "https://") {
		mediaUrl = attr.Val
	} else {
		mediaUrl = ctx.RootUrl + attr.Val
	}

	data, err := httpGet(mediaUrl)
	if err != nil {
		return err
	}
	mediaId, mediaFilename, err := insertMedia([]byte(data), ctx.Db)
	if err != nil {
		return err
	}
	attr.Val = mediaFilename

	_, ok := ctx.AlreadyRelated[MediaPage{mediaId, ctx.PageId}]
	if !ok {
		query := `INSERT INTO media_web_pages (media_id, web_page_id) VALUES (?, ?)`
		_, err = ctx.Db.Exec(query, mediaId, ctx.PageId)
		if err != nil {
			return err
		}

		ctx.AlreadyRelated[MediaPage{mediaId, ctx.PageId}] = true
	}
	return nil
}

func parseHtml(ctx *HtmlParsingContext, node *html.Node) error {
	if node.Type == html.ElementNode {
		if node.Data == "img" || node.Data == "script" {
			for attr_i, attr := range node.Attr {
				if attr.Key == "src" {
					err := processMediaLink(ctx, node, attr_i)
					if err != nil {
						return err
					}
				}
			}
		} else if node.Data == "link" {
			for attr_i, attr := range node.Attr {
				if attr.Key == "icon" || attr.Key == "stylesheet" {
					err := processMediaLink(ctx, node, attr_i)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		err := parseHtml(ctx, c)
		if err != nil {
			return err
		}
	}

	return nil
}

func queryAndSaveWebUrl(urlString string, dbContext *DbContext) error {
	text, err := httpGet(urlString)
	if err != nil {
		return err
	}

	urlParsed, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}

	dbContext.Mut.Lock()
	defer dbContext.Mut.Unlock()
	db, err := sql.Open("sqlite3", dbContext.ConnectionString)
	defer db.Close()

	var pageId int
	query := `INSERT INTO web_pages (url) VALUES (?)`
	_, err = db.Exec(query, urlString)
	if err != nil {
		return err
	}

	pageId, err = getLastAutoincrementIndex(db)
	if err != nil {
		return err
	}

	rootUrl := urlParsed.Scheme + "://" + urlParsed.Hostname()
	root, err := html.Parse(strings.NewReader(text))
	if err != nil {
		panic(err)
	}

	htmlCtx := HtmlParsingContext{RootUrl: rootUrl, PageId: pageId, Db: db, AlreadyRelated: make(map[MediaPage]bool)}

	err = parseHtml(&htmlCtx, root)
	if err != nil {
		return err
	}

	textModified := bytes.NewBuffer(nil)
	err = html.Render(textModified, root)
	if err != nil {
		return err
	}

	query = `INSERT INTO web_pages_data (page_id, data, timestamp)
		VALUES (?, ?, ?);`
	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	_, err = db.Exec(query, pageId, textModified.Bytes(), timestamp)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	return nil
}
