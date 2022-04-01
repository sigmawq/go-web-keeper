package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/html"
	"log"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	UrlJobs               []UrlJob `json:"url_jobs"`
	EraseStorageOnStartup bool     `json:"erase_storage_on_startup"`
}

type UrlJob struct {
	Url                string
	QueryPeriodSeconds int `json:"query_period_seconds"`
	RetryPeriodSeconds int `json:"retry_period_seconds"`
}

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

	if config.EraseStorageOnStartup {
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

func parseHtml(node *html.Node, rootUrl string, pageId int, db *sql.DB) error {
	if node.Type == html.ElementNode {
		if node.Data == "img" || node.Data == "script" {
			for attr_i, attr := range node.Attr {
				if attr.Key == "src" {
					var mediaUrl string
					if strings.HasPrefix(attr.Val, "http://") || strings.HasPrefix(attr.Val, "https://") {
						mediaUrl = attr.Val
					} else {
						mediaUrl = rootUrl + attr.Val
					}

					log.Printf("%v", mediaUrl)

					data, err := httpGet(mediaUrl)
					if err == nil {

						filename := path.Base(mediaUrl)
						node.Attr[attr_i].Val = filename

						query := `INSERT INTO media (name, data) VALUES (?, ?)`
						_, err := db.Exec(query, filename, data)
						if err != nil {
							return err
						}

						mediaId, err := getLastAutoincrementIndex(db)
						if err != nil {
							return err
						}

						query = `INSERT INTO media_web_pages (media_id, web_page_id) VALUES (?, ?)`
						_, err = db.Exec(query, mediaId, pageId)
						if err != nil {
							fmt.Println(err)
							return err
						}
					} else {
						log.Printf("Failed to get %v. Node will be left unparsed", mediaUrl)

					}
				}
			}
		} else if node.Data == "link" {

		}
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		err := parseHtml(c, rootUrl, pageId, db)
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

	err = parseHtml(root, rootUrl, pageId, db)
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
		fmt.Println(err)
		return err
	}

	if err != nil {
		return err
	}

	return nil
}
