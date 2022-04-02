package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"time"
	// "bufio"
	"archive/zip"
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	// "log"
)

type PageData struct {
	Id        int
	Data      string
	Timestamp int
	Url       string
	Media     []Media // TODO: Query media indicies first and then make a final pass to query media data. This will help avoid copies
}

type Media struct {
	Name string
	Data []byte
}

func queryUrlDataInRange(dbContext *DbContext, url string, from time.Time, to time.Time) ([]PageData, error) {
	dbContext.Mut.Lock()
	defer dbContext.Mut.Unlock()

	db, err := sql.Open("sqlite3", dbContext.ConnectionString)
	defer db.Close()

	query := `select page_id, data, timestamp from web_pages_data
		inner join web_pages on web_pages_data.page_id = web_pages.id
		where web_pages.url = ? and timestamp > ? and timestamp < ?`
	rows, err := db.Query(query, url, strconv.FormatInt(from.UTC().UnixNano(), 10), strconv.FormatInt(to.UTC().UnixNano(), 10))
	if err != nil {
		return nil, err
	}

	var pages []PageData
	i := 0
	for rows.Next() {
		pages = append(pages, PageData{})
		page := &pages[i]
		pages[i].Url = url
		err := rows.Scan(&page.Id, &page.Data, &page.Timestamp)
		if err != nil {
			return nil, err
		}

		query := `SELECT name, data FROM media
			INNER JOIN media_web_pages ON media_web_pages.media_id = media.id
			WHERE media_web_pages.web_page_id = ?`
		rows2, err := db.Query(query, pages[i].Id)
		if err != nil {
			return nil, err
		}

		k := 0
		for rows2.Next() {

			pages[i].Media = append(pages[i].Media, Media{})
			media := &pages[i].Media[k]
			rows2.Scan(&media.Name, &media.Data)

			k += 1
		}

		i += 1
	}

	return pages, nil
}

func zipifyPages(pagesData []PageData) ([]byte, error) {
	buffer := new(bytes.Buffer)
	zipf := zip.NewWriter(buffer)

	for i, pageData := range pagesData {
		refinedUrl, err := url.Parse(pageData.Url)
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("%v_%v_%v.zip", i, pageData.Timestamp, refinedUrl.Hostname()) // <idx>_<hostname><path>_<timestamp>
		if err != nil {
			return nil, err
		}

		zipfNested, err := zipf.Create(name)
		if err != nil {
			return nil, err
		}
		zipfNestedW := zip.NewWriter(zipfNested)

		htmlWriter, err := zipfNestedW.Create("index.html")
		if err != nil {
			return nil, err
		}
		_, err = htmlWriter.Write([]byte(pageData.Data))
		if err != nil {
			panic(err)
		}
		// fmt.Printf("A: %v\n", pageData)

		for _, media := range pageData.Media {
			if media.Name == "" {
				panic("Empty name")
			}
			fileWriter, err := zipfNestedW.Create(media.Name)
			if err != nil {
				return nil, err
			}
			_, err = fileWriter.Write([]byte(media.Data))
			if err != nil {
				panic(err)
			}
		}

		err = zipfNestedW.Close()
		if err != nil {
			return nil, err
		}
	}

	err := zipf.Close()
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
