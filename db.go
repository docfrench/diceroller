package main

import (
	"database/sql"

	_ "modernc.org/sqlite" // blank import registers the driver with database/sql
)

var db *sql.DB

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", "./data/rolls.db")
	if err != nil {
		return err
	}

	// SQLite only allows one writer at a time by default; WAL mode
	// lets reads happen concurrently with a write instead of blocking
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS rolls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			character TEXT,
			reason TEXT,
			notation TEXT,
			difficulty INTEGER,
			successes INTEGER,
			rolls TEXT NOT NULL,
			total INTEGER NOT NULL DEFAULT 0,
			modifier INTEGER NOT NULL DEFAULT 0,
			roll_type TEXT NOT NULL DEFAULT 'storyteller',
			rolled_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}
