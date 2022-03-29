package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"time"
	// "bufio"
	"archive/zip"
	"bytes"
	"net/url"
	"strconv"
	"fmt"
	// "log"
)

type PageData struct {
	Id int
	Url string
	Data string
	Timestamp int
}

func queryUrlDataInRange(dbContext *DbContext, url string, from time.Time, to time.Time) ([]PageData, error) {
	dbContext.Mut.Lock()
	defer dbContext.Mut.Unlock()

	query := `SELECT * from web_pages_data
	WHERE url = ? AND timestamp >= ? AND timestamp <= ?`

	db, err := sql.Open("sqlite3", dbContext.ConnectionString)
	defer db.Close()

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}	

	rows, err := stmt.Query(url, strconv.FormatInt(from.UTC().UnixNano(), 10), strconv.FormatInt(to.UTC().UnixNano(), 10))
	if err != nil {
		return nil, err
	}

	var pages []PageData
	i := 0
	for rows.Next() {
		pages = append(pages, PageData{})
		err := rows.Scan(&pages[i].Id, &pages[i].Url, &pages[i].Data, &pages[i].Timestamp)
		if err != nil {
			return nil, err
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
		name := fmt.Sprintf("%v_%v%v_%v.zip", i, pageData.Timestamp, refinedUrl.Hostname(), refinedUrl.Path) // <idx>_<hostname><path>_<timestamp>
		zipfInternalWriter, err := zipf.Create(name)
		if err != nil {
			return nil, err
		}

		zipfInternal := zip.NewWriter(zipfInternalWriter)
		htmlWriter, err := zipfInternal.Create("index.html")
		if err != nil {
			return nil, err
		}
		htmlWriter.Write([]byte(pageData.Data))

		zipfInternal.Close()
	} 

	zipf.Close()

	return buffer.Bytes(), nil
}