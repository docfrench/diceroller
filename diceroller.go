package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite" // blank import registers the driver with database/sql
)

var db *sql.DB

var diceNotation = regexp.MustCompile(`^(\d+)d(\d+)([+-]\d+)?$`)

type RollResult struct {
	Display    string
	LogLine    string
	Rolls      []int
	RollType   string
	Successes  *int // nil when the system has no success-count concept
	Difficulty *int
	Notation   string
}

type Hub struct {
	clients    map[chan string]bool
	broadcast  chan string
	register   chan chan string
	unregister chan chan string
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[chan string]bool),
		broadcast:  make(chan string),
		register:   make(chan chan string),
		unregister: make(chan chan string),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case ch := <-h.register:
			h.clients[ch] = true

		case ch := <-h.unregister:
			if _, ok := h.clients[ch]; ok {
				delete(h.clients, ch)
				close(ch)
			}

		case msg := <-h.broadcast:
			for ch := range h.clients {
				select {
				case ch <- msg:
					// delivered
				default:
					// this client's channel is full/stuck — drop it
					// rather than let one slow reader block everyone
					delete(h.clients, ch)
					close(ch)
				}
			}
		}
	}
}

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
			roll_type TEXT NOT NULL DEFAULT 'storyteller',
			rolled_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func HomePage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("dice.html")
	if err != nil {
		log.Print("template parsing error: ", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, nil); err != nil {
		log.Print("template executing error: ", err)
	}
}

func rollD20(r *http.Request, character, reason string) (RollResult, error) {
	modifier, err := strconv.Atoi(r.FormValue("modifier"))
	if err != nil {
		modifier = 0
	}

	roll := rand.Intn(20) + 1
	total := roll + modifier

	note := ""
	switch {
	case roll == 20:
		note = `<span style="color:var(--good)"><strong>*~* Natural 20! *~*</strong></span>`
	case roll == 1:
		note = `<span style="color:var(--oxblood-bright)"><strong>......Natural 1......</strong></span>`
	default:
		// no special note
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<p><div align="center">%s rolled <strong>d20%+d</strong><br><br>Roll: %d<br><br>Total: <strong>%d</strong><br><br>%s</div></p>`,
		character, modifier, roll, total, note)
	logLine := fmt.Sprintf("%s rolled 1d20+%d to %s: %d + %d = %d total (%s)", character, modifier, reason, roll, modifier, total, time.Now().Format("03:04PM"))

	return RollResult{Display: b.String(), LogLine: logLine, Rolls: []int{roll}, RollType: "d20"}, nil
}

func rollD100(r *http.Request, character, reason string) (RollResult, error) {
	target, err := strconv.Atoi(r.FormValue("target_pct"))
	if err != nil || target < 1 || target > 100 {
		return RollResult{}, fmt.Errorf("invalid target: %s", r.FormValue("target_pct"))
	}

	roll := rand.Intn(100) + 1
	var outcome, display string

	if roll <= target {
		outcome = "Success"
		display = fmt.Sprintf(`<div align="center"><p>%s rolled <strong>d100</strong> vs target %d%%<br><br><span style="color:var(--good)">Roll: %d<br><br><strong>%s</strong></span></p></div>`,
			character, target, roll, outcome)
	} else {
		outcome = "Failure"
		display = fmt.Sprintf(`<div align="center"><p>%s rolled <strong>d100</strong> vs target %d%%<br><br><span style="color:var(--oxblood-bright)">Roll: %d<br><br><strong>%s</strong></span></p></div>`,
			character, target, roll, outcome)
	}

	logLine := fmt.Sprintf("%s rolled %d against %d to %s: %s (%s)", character, roll, target, reason, outcome, time.Now().Format("03:04PM"))

	return RollResult{
		Display:  display,
		LogLine:  logLine,
		Rolls:    []int{roll},
		RollType: "d100",
	}, nil
}

func rollStoryteller(r *http.Request, character, reason string) (RollResult, error) {
	notation := r.FormValue("notation")
	difficulty, err := strconv.Atoi(r.FormValue("difficulty"))
	if err != nil || difficulty < 2 || difficulty > 10 {
		return RollResult{}, fmt.Errorf("invalid difficulty: %s", r.FormValue("difficulty"))
	}

	matches := diceNotation.FindStringSubmatch(notation)
	if matches == nil {
		return RollResult{}, fmt.Errorf("invalid notation: %s", notation)
	}
	count, _ := strconv.Atoi(matches[1])
	sides, _ := strconv.Atoi(matches[2])

	rolls := make([]int, count)
	successes := 0
	for i := 0; i < count; i++ {
		roll := rand.Intn(sides) + 1
		rolls[i] = roll
		if roll >= difficulty {
			successes++
		}
		if roll == 1 {
			successes--
		}
	}

	var b strings.Builder

	if reason != "" {
		fmt.Fprintf(&b, `<p class="roll-reason">%s</p>`, reason)
	}

	fmt.Fprintf(&b, `<p>%s rolled <strong>%d</strong> dice against difficulty %d → <br><br> <b>Rolls: %v</b><br><br>`,
		character, count, difficulty, rolls)

	if successes > 0 {
		fmt.Fprintf(&b, `<span style="color:var(--good)"><strong>%d successes</strong></span></p>`, successes)
	} else {
		fmt.Fprintf(&b, `<span style="color:var(--oxblood-bright)"><strong>%d successes</strong></span></p>`, successes)
	}

	sortedRolls := make([]int, count)
	copy(sortedRolls, rolls)
	sort.Sort(sort.Reverse(sort.IntSlice(sortedRolls)))

	for i := 0; i < count; i++ {
		switch {
		case sortedRolls[i] >= difficulty:
			fmt.Fprintf(&b, `<span style="color:var(--good)">%d success</span><br> `, sortedRolls[i])
		case sortedRolls[i] == 1:
			fmt.Fprintf(&b, `<span style="color:var(--oxblood-bright)">** %d botch **</span><br> `, sortedRolls[i])
		default:
			fmt.Fprintf(&b, `<span style="color:var(--parchment-dim)">%d failure</span><br> `, sortedRolls[i])
		}
	}

	logLine := fmt.Sprintf("%s rolled %d dice to %s: %d successes (%s)", character, count, reason, successes, time.Now().Format("03:04PM"))

	return RollResult{
		Display:    b.String(),
		LogLine:    logLine,
		Rolls:      rolls,
		RollType:   "storyteller",
		Notation:   notation,
		Difficulty: &difficulty,
		Successes:  &successes,
	}, nil
}

func RollHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("password") != os.Getenv("TABLE_PASS") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `<p>Wrong passphrase.</p>`)
			return
		}

		character := r.FormValue("character")
		reason := r.FormValue("reason")
		rollType := r.FormValue("roll_type")

		var result RollResult
		var err error

		switch rollType {
		case "d20":
			result, err = rollD20(r, character, reason)
		case "d100":
			result, err = rollD100(r, character, reason)
		default:
			result, err = rollStoryteller(r, character, reason)
		}

		if err != nil {
			_, _ = fmt.Fprintf(w, `<p>%v</p>`, err)
			return
		}

		_, _ = fmt.Fprint(w, result.Display)

		rollsJSON, err := json.Marshal(result.Rolls)
		if err != nil {
			log.Print("error marshaling rolls: ", err)
		}

		var difficulty sql.NullInt64
		if result.Difficulty != nil {
			difficulty = sql.NullInt64{Int64: int64(*result.Difficulty), Valid: true}
		}
		var successes sql.NullInt64
		if result.Successes != nil {
			successes = sql.NullInt64{Int64: int64(*result.Successes), Valid: true}
		}

		_, err = db.Exec(
			`INSERT INTO rolls (character, reason, roll_type, notation, difficulty, successes, rolls) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			character, reason, result.RollType, result.Notation, difficulty, successes, string(rollsJSON),
		)
		if err != nil {
			log.Print("error saving roll: ", err)
		}
		htmlLine := fmt.Sprintf(`<div class="roll-entry">%s [%s]</div>`, result.LogLine, result.RollType)
		select {
		case hub.broadcast <- htmlLine:
		default:
		}
	}
}

func recentRolls(limit int) ([]string, error) {
	rows, err := db.Query(
		`SELECT character, reason, successes, rolled_at, roll_type FROM rolls ORDER BY rolled_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Print("error closing rows: ", err)
		}
	}()

	var lines []string
	for rows.Next() {
		var character, reason string
		var successes sql.NullInt64
		var rolledAt time.Time
		var rollType string
		if err := rows.Scan(&character, &reason, &successes, &rolledAt, &rollType); err != nil {
			return nil, err
		}

		successCount := 0
		if successes.Valid {
			successCount = int(successes.Int64)
		}
		lines = append(lines, formatRollLineAt(character, reason, successCount, rollType, rolledAt))
	}
	return lines, rows.Err()
}

func formatRollLineAt(character, reason string, successes int, rollType string, t time.Time) string {
	label := character
	if label == "" {
		label = "Someone"
	}

	action := reason
	if action == "" {
		action = "a roll"
	}

	successWord := "successes"
	if successes == 1 {
		successWord = "success"
	}

	return fmt.Sprintf("%s rolled %s: %d %s (%s) [%s]",
		label, action, successes, successWord, t.Format("03:04PM"), rollType)
}

func EventsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		history, err := recentRolls(20)
		if err != nil {
			log.Print("error loading roll history: ", err)
		} else {
			// reverse to chronological order (oldest first) since the query
			// was DESC for LIMIT to grab the *most recent* N correctly
			for i := len(history) - 1; i >= 0; i-- {
				_, _ = fmt.Fprintf(w, "data: <div class=\"roll-entry\">%s</div>\n\n", history[i])
			}
			flusher.Flush()
		}

		ch := make(chan string)
		hub.register <- ch

		defer func() {
			hub.unregister <- ch
		}()

		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					// hub closed this channel (dropped us as a slow client)
					return
				}
				_, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()

			case <-r.Context().Done():
				// client closed the tab / connection dropped
				return
			}
		}
	}
}

func main() {
	if err := godotenv.Load(); err != nil {

		log.Print("Error loading .env file")
	}
	if err := initDB(); err != nil {
		log.Fatal("failed to open database: ", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Print("error closing database: ", err)
		}
	}()
	hub := NewHub()
	go hub.Run()
	fmt.Println("Dice Roller is listening on port 8080")
	http.HandleFunc("/", HomePage)
	http.HandleFunc("/roll", RollHandler(hub))
	http.HandleFunc("/events", EventsHandler(hub))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
