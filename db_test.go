package main

import (
	"path/filepath"
	"testing"
)

func TestInitDB(t *testing.T) {
	// Use an isolated temporary database.
	dbPath = filepath.Join(t.TempDir(), "rolls.db")

	// Clean up the global database connection and restore the default path.
	t.Cleanup(func() {
		if db != nil {
			_ = db.Close()
			db = nil
		}
		dbPath = "/data/rolls.db"
	})

	// Initialize the database.
	if err := initDB(); err != nil {
		t.Fatalf("initDB() failed: %v", err)
	}

	if db == nil {
		t.Fatal("initDB() returned successfully but db is nil")
	}

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		t.Fatalf("database Ping() failed: %v", err)
	}

	// Verify the rolls table exists.
	var tableName string

	err := db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		AND name = 'rolls'
	`).Scan(&tableName)

	if err != nil {
		t.Fatalf("failed to find rolls table: %v", err)
	}

	if tableName != "rolls" {
		t.Fatalf("expected rolls table, got %q", tableName)
	}

	// Verify that we can actually insert and retrieve a roll.
	_, err = db.Exec(`
		INSERT INTO rolls (
			character,
			reason,
			notation,
			difficulty,
			successes,
			rolls,
			total,
			modifier,
			roll_type
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		"Test Character",
		"Unit test",
		"5d10",
		6,
		3,
		"8,7,6,4,2",
		17,
		0,
		"storyteller",
	)

	if err != nil {
		t.Fatalf("failed to insert test roll: %v", err)
	}

	var (
		character string
		reason    string
		total     int
	)

	err = db.QueryRow(`
		SELECT character, reason, total
		FROM rolls
		LIMIT 1
	`).Scan(&character, &reason, &total)

	if err != nil {
		t.Fatalf("failed to retrieve test roll: %v", err)
	}

	if character != "Test Character" {
		t.Errorf("expected character %q, got %q",
			"Test Character", character)
	}

	if reason != "Unit test" {
		t.Errorf("expected reason %q, got %q",
			"Unit test", reason)
	}

	if total != 17 {
		t.Errorf("expected total %d, got %d", 17, total)
	}
}
