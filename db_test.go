package main

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	// Create an isolated temporary directory for the test database.
	tempDir := t.TempDir()
	dbPath = filepath.Join(tempDir, "rolls.db")

	// Reset the global database connection when the test finishes.
	t.Cleanup(func() {
		if db != nil {
			defer func() {
				if err := db.Close(); err != nil {
					log.Print("error closing database: ", err)
				}
			}()
			db = nil
		}
		dbPath = "/data/rolls.db"
	})

	// Initialize the database.
	if err := initDB(); err != nil {
		t.Fatalf("initDB() returned an error: %v", err)
	}

	if db == nil {
		t.Fatal("initDB() did not initialize the database")
	}

	// Verify that the database is actually usable.
	if err := db.Ping(); err != nil {
		t.Fatalf("database Ping() failed: %v", err)
	}

	// Verify that the rolls table was created.
	var tableName string
	err := db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'rolls'
	`).Scan(&tableName)

	if err != nil {
		t.Fatalf("failed to query for rolls table: %v", err)
	}

	if tableName != "rolls" {
		t.Fatalf("expected rolls table, got %q", tableName)
	}

	// Verify the expected columns exist.
	rows, err := db.Query(`PRAGMA table_info(rolls)`)
	if err != nil {
		t.Fatalf("failed to inspect rolls table: %v", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Print("error closing rows: ", err)
		}
	}()

	expectedColumns := map[string]bool{
		"id":         false,
		"character":  false,
		"reason":     false,
		"notation":   false,
		"difficulty": false,
		"successes":  false,
		"rolls":      false,
		"total":      false,
		"modifier":   false,
		"roll_type":  false,
		"rolled_at":  false,
	}

	for rows.Next() {
		var (
			cid          int
			name         string
			columnType   string
			notNull      int
			defaultValue any
			primaryKey   int
		)

		if err := rows.Scan(
			&cid,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("failed to scan table info: %v", err)
		}

		if _, ok := expectedColumns[name]; ok {
			expectedColumns[name] = true
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("error reading table info: %v", err)
	}

	for column, found := range expectedColumns {
		if !found {
			t.Errorf("expected column %q was not found", column)
		}
	}

	// Verify that initDB is idempotent. Calling it again should not
	// produce an error because the table already exists.
	if err := initDB(); err != nil {
		t.Fatalf("second initDB() returned an error: %v", err)
	}

	// Verify the database file was actually created.
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
}
