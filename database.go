package main

import (
	"database/sql"
	"log"
	_ "modernc.org/sqlite"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite", "serverwatch.db")
	if err != nil {
		log.Fatal(err)
	}

	db.Exec(`CREATE TABLE IF NOT EXISTS endpoints(
		url TEXT PRIMARY KEY,
		first_seen TEXT,
		last_seen TEXT
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS findings(
		type TEXT,
		value TEXT,
		severity TEXT,
		created TEXT
	)`)
}
