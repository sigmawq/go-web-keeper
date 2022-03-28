package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	UrlJobs []UrlJob `json:"url_jobs"`
}

type UrlJob struct {
	Url                string
	QueryPeriodSeconds int `json:"query_period_seconds"`
	RetryPeriodSeconds int `json:"retry_period_seconds"`
}

func initialize(connectionString string) (Config, DbContext, error) {
	query := `CREATE TABLE IF NOT EXISTS web_pages_data (
		id INTEGER NOT NULL PRIMARY KEY, 
		url TEXT NOT NULL,
		data TEXT NOT NULL,
		timestamp INTEGER NOT NULL);`

	db, err := sql.Open("sqlite3", connectionString)

	// TODO: replace panics

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
		valid := true
		if urlJob.Url == "" {
			fmt.Printf("URL job %v has no URL specified and will be ignored\n", i)
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

		if !valid {
			jobsToDelete = append(jobsToDelete, i)
		}
	}

	for _, i := range jobsToDelete {
		config.UrlJobs = append(config.UrlJobs[:i], config.UrlJobs[i+1:]...)
	}

	var dbContext DbContext
	dbContext.ConnectionString = connectionString

	return config, dbContext, nil
}

func queryAndStoreRoutine(urlJobId int, urlJob UrlJob, dbContext *DbContext) {
	log.Printf("[Job ID: %v | URL: %v]: Start", urlJobId, urlJob.Url) // here
	for {
		log.Printf("[Job ID: %v | URL: %v]: Performing request..", urlJobId, urlJob.Url) // here
		err := queryAndSaveWebUrl(urlJob.Url, dbContext)
		if err != nil {
			log.Printf("[Job ID: %v | URL: %v]: Requesting URL failed: %v", urlJobId, urlJob.Url, err) // here
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

func queryAndSaveWebUrl(url string, dbContext *DbContext) error {
	text, err := httpGet(url)
	if err != nil {
		return err
	}

	query := `INSERT INTO web_pages_data (url, data, timestamp)
		VALUES (?, ?, ?);`

	dbContext.Mut.Lock()
	defer dbContext.Mut.Unlock()
	db, err := sql.Open("sqlite3", dbContext.ConnectionString)
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
