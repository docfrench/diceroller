package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub() returned nil")
	}

	if hub.clients == nil {
		t.Fatal("hub.clients is nil")
	}

	if hub.broadcast == nil {
		t.Fatal("hub.broadcast is nil")
	}

	if hub.register == nil {
		t.Fatal("hub.register is nil")
	}

	if hub.unregister == nil {
		t.Fatal("hub.unregister is nil")
	}
}

func TestFormatRollLineAtD20(t *testing.T) {
	testTime := time.Date(
		2026, time.September, 25,
		15, 30, 0, 0,
		time.UTC,
	)

	tests := []struct {
		name      string
		character string
		reason    string
		notation  string
		modifier  int
		total     int
		rollType  string
		want      string
	}{
		{
			name:      "with modifier",
			character: "Alice",
			reason:    "Attack",
			notation:  "1d20+5",
			modifier:  5,
			total:     18,
			rollType:  "d20",
			want:      "Alice rolled Attack (1d20+5): 18 (03:30PM) [d20]",
		},
		{
			name:      "without modifier",
			character: "Bob",
			reason:    "Perception",
			notation:  "1d20",
			modifier:  0,
			total:     12,
			rollType:  "d20",
			want:      "Bob rolled Perception (1d20): 12 (03:30PM) [d20]",
		},
		{
			name:      "empty character",
			character: "",
			reason:    "Attack",
			notation:  "1d20",
			modifier:  0,
			total:     10,
			rollType:  "d20",
			want:      "Someone rolled Attack (1d20): 10 (03:30PM) [d20]",
		},
		{
			name:      "empty reason",
			character: "Alice",
			reason:    "",
			notation:  "1d20",
			modifier:  0,
			total:     10,
			rollType:  "d20",
			want:      "Alice rolled a roll (1d20): 10 (03:30PM) [d20]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRollLineAt(
				tt.character,
				tt.reason,
				tt.notation,
				0,
				tt.rollType,
				testTime,
				tt.modifier,
				tt.total,
				nil,
			)

			if got != tt.want {
				t.Errorf("formatRollLineAt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRollLineAtD100(t *testing.T) {
	testTime := time.Date(
		2026, time.September, 25,
		15, 30, 0, 0,
		time.UTC,
	)

	rolls, err := json.Marshal([]int{73})
	if err != nil {
		t.Fatal(err)
	}

	got := formatRollLineAt(
		"Alice",
		"Investigation",
		"1d100",
		0,
		"d100",
		testTime,
		0,
		73,
		rolls,
	)

	want := "Alice rolled Investigation (1d100): 73 (03:30PM) [d100]"

	if got != want {
		t.Errorf("formatRollLineAt() = %q, want %q", got, want)
	}
}

func TestFormatRollLineAtD100InvalidJSON(t *testing.T) {
	testTime := time.Date(
		2026, time.September, 25,
		15, 30, 0, 0,
		time.UTC,
	)

	got := formatRollLineAt(
		"Alice",
		"Investigation",
		"1d100",
		0,
		"d100",
		testTime,
		0,
		0,
		json.RawMessage(`not valid json`),
	)

	want := "Alice rolled Investigation (1d100): 0 (03:30PM) [d100]"

	if got != want {
		t.Errorf("formatRollLineAt() = %q, want %q", got, want)
	}
}

func TestFormatRollLineAtStoryteller(t *testing.T) {
	testTime := time.Date(
		2026, time.September, 25,
		15, 30, 0, 0,
		time.UTC,
	)

	tests := []struct {
		name      string
		successes int
		want      string
	}{
		{
			name:      "one success",
			successes: 1,
			want:      "Alice rolled Attack (storyteller): 1 success (03:30PM) [storyteller]",
		},
		{
			name:      "multiple successes",
			successes: 3,
			want:      "Alice rolled Attack (storyteller): 3 successes (03:30PM) [storyteller]",
		},
		{
			name:      "zero successes",
			successes: 0,
			want:      "Alice rolled Attack (storyteller): 0 successes (03:30PM) [storyteller]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRollLineAt(
				"Alice",
				"Attack",
				"5d10",
				tt.successes,
				"storyteller",
				testTime,
				0,
				0,
				nil,
			)

			if got != tt.want {
				t.Errorf("formatRollLineAt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRollLineAtDefaultRollType(t *testing.T) {
	testTime := time.Date(
		2026, time.September, 25,
		15, 30, 0, 0,
		time.UTC,
	)

	got := formatRollLineAt(
		"Alice",
		"Attack",
		"5d10",
		2,
		"unknown",
		testTime,
		0,
		0,
		nil,
	)

	want := "Alice rolled Attack (unknown): 2 successes (03:30PM) [unknown]"

	if got != want {
		t.Errorf("formatRollLineAt() = %q, want %q", got, want)
	}
}

func TestRecentRolls(t *testing.T) {
	dbPath = filepath.Join(t.TempDir(), "rolls.db")

	t.Cleanup(func() {
		if db != nil {
			_ = db.Close()
			db = nil
		}
		dbPath = "/data/rolls.db"
	})

	if err := initDB(); err != nil {
		t.Fatalf("initDB() failed: %v", err)
	}

	// Insert two known rolls.
	_, err := db.Exec(`
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
		"Alice",
		"Attack",
		"5d10",
		6,
		3,
		`[8,7,6,4,2]`,
		25,
		0,
		"storyteller",
	)
	if err != nil {
		t.Fatalf("failed to insert first roll: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO rolls (
			character,
			reason,
			notation,
			rolls,
			total,
			modifier,
			roll_type
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		"Bob",
		"Attack",
		"1d20+2",
		`[17]`,
		19,
		2,
		"d20",
	)
	if err != nil {
		t.Fatalf("failed to insert second roll: %v", err)
	}

	lines, err := recentRolls(10)
	if err != nil {
		t.Fatalf("recentRolls() failed: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 roll lines, got %d", len(lines))
	}

	foundAlice := false
	foundBob := false

	for _, line := range lines {
		if strings.Contains(line, "Alice") {
			foundAlice = true
		}

		if strings.Contains(line, "Bob") {
			foundBob = true
		}
	}

	if !foundAlice {
		t.Error("did not find Alice roll")
	}

	if !foundBob {
		t.Error("did not find Bob roll")
	}
}

func TestRecentRollsLimit(t *testing.T) {
	dbPath = filepath.Join(t.TempDir(), "rolls.db")

	t.Cleanup(func() {
		if db != nil {
			_ = db.Close()
			db = nil
		}
		dbPath = "/data/rolls.db"
	})

	if err := initDB(); err != nil {
		t.Fatalf("initDB() failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, err := db.Exec(`
			INSERT INTO rolls (
				character,
				reason,
				notation,
				rolls,
				total,
				modifier,
				roll_type
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`,
			"Character",
			"Test",
			"1d20",
			`[10]`,
			10,
			0,
			"d20",
		)

		if err != nil {
			t.Fatalf("failed to insert roll %d: %v", i, err)
		}
	}

	lines, err := recentRolls(3)
	if err != nil {
		t.Fatalf("recentRolls() failed: %v", err)
	}

	if len(lines) != 3 {
		t.Errorf("expected 3 rolls, got %d", len(lines))
	}
}
