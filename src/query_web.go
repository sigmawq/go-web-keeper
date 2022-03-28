package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
	"time"
	"encoding/json"
	"os"
)

type Config struct {
	UrlJobs []UrlJob `json:"url_jobs"`
}

type UrlJob struct {
	Url string
	QueryPeriodSeconds int `json:"query_period_seconds"`
	RetryPeriodSeconds int `json:"retry_period_seconds"`
}

func initialize(connectionString string) (Config, error) {
	query := `CREATE TABLE IF NOT EXISTS web_pages_data (
		id INTEGER NOT NULL PRIMARY KEY, 
		url TEXT NOT NULL,
		data TEXT NOT NULL,
		timestamp INTEGER NOT NULL);`

	db, err := sql.Open("sqlite3", connectionString)

	if err != nil {
		panic(err)
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		panic(err)
	}

	_, err = stmt.Exec()
	if err != nil {
		panic(err)
	}

	db.Close()

	data, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}

	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	jobsToDelete := make([]int, 0, 12)
	for i, urlJob := range config.UrlJobs {
		if urlJob.Url == "" {
			fmt.Printf("URL job %v has no URL specified and will be ignored\n", i)
			jobsToDelete = append(jobsToDelete, i)
		}
	}

	for _, i := range jobsToDelete {
		config.UrlJobs = append(config.UrlJobs[:i], config.UrlJobs[i+1:]...)
	}

	return config, nil
}

func queryAndSaveWebUrl(url string, dbConnectionString string) error {
	text, err := httpGet(url)
	if err != nil {
		return err
	}

	query := `INSERT INTO web_pages_data (url, data, timestamp)
		VALUES (?, ?, ?);`
	db, err := sql.Open("sqlite3", dbConnectionString)
	if err != nil {
		return err
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		fmt.Println(err)
		return err
	}

	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	_, err = stmt.Exec(url, text, timestamp)
	if err != nil {
		fmt.Println(err)
		return err
	}

	stmt.Close()
	db.Close()

	if err != nil {
		return err
	}

	return nil
}
