package main

import (
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"net/http"
	// "github.com/mattn/go-sqlite3"
)

func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func getLastAutoincrementIndex(db *sql.DB) (int, error) {
	query := `SELECT last_insert_rowid()`
	stmt, err := db.Prepare(query)
	if err != nil {
		return 0, err
	}

	rows, err := stmt.Query()
	if err != nil {
		return 0, err
	}

	id := -1
	for rows.Next() {
		rows.Scan(&id)
	}

	if id == -1 {
		return 0, errors.New("autoincrement query returned no results")
	}

	return id, nil
}
