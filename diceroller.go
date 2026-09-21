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
	"time"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite" // blank import registers the driver with database/sql
)

var db *sql.DB

var diceNotation = regexp.MustCompile(`^(\d+)d(\d+)([+-]\d+)?$`)

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
	db, err = sql.Open("sqlite", "/data/rolls.db")
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
			notation TEXT NOT NULL,
			difficulty INTEGER NOT NULL,
			successes INTEGER NOT NULL,
			rolls TEXT NOT NULL,
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

func RollHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tablePassphrase := os.Getenv("TABLE_PASS")
		if r.FormValue("password") != tablePassphrase {
			w.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(w, `<p>Wrong passphrase.</p>`)
			return
		}

		notation := r.FormValue("notation")
		reason := r.FormValue("reason")
		character := r.FormValue("character")
		difficulty, err := strconv.Atoi(r.FormValue("difficulty"))
		if err != nil || difficulty < 2 || difficulty > 10 {
			_, _ = fmt.Fprintf(w, `<p>Invalid difficulty: %s</p>`, r.FormValue("difficulty"))
			return
		}

		matches := diceNotation.FindStringSubmatch(notation)
		if matches == nil {
			_, _ = fmt.Fprintf(w, `<p>Invalid notation: %s</p>`, notation)
			return
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
				successes-- // botch in Storyteller system
			}
		}

		if reason != "" {
			_, _ = fmt.Fprintf(w, `<p class="roll-reason">%s</p>`, reason)
		}

		_, _ = fmt.Fprintf(w, `<p>%s rolled <strong>%d</strong> dice against difficulty %d → <br><br> <b>Rolls: %v</b><br><br>`,
			character, count, difficulty, rolls)
		if successes > 0 {
			_, _ = fmt.Fprintf(w, `<span style="color:var(--good)"><strong>%d successes</strong></span></p>`, successes)
		} else {
			_, _ = fmt.Fprintf(w, `<span style="color:var(--oxblood-bright)"><strong>%d successes</strong></span></p>`, successes)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(rolls)))
		for i := 0; i < count; i++ {
			if rolls[i] >= difficulty {
				_, _ = fmt.Fprintf(w, `<span style="color:var(--good)">%d success</span><br> `, rolls[i])
			} else if rolls[i] == 1 {
				_, _ = fmt.Fprintf(w, `<span style="color:var(--oxblood-bright)">** %d botch **</span><br> `, rolls[i])
			} else {
				_, _ = fmt.Fprintf(w, `<span style="color:var(--parchment-dim)">%d failure</span><br> `, rolls[i])
			}
		}
		rollsJSON, err := json.Marshal(rolls)
		if err != nil {
			log.Print("error marshaling rolls: ", err)
		}
		_, err = db.Exec(
			`INSERT INTO rolls (character, reason, notation, difficulty, rolls, successes) VALUES (?, ?, ?, ?, ?, ?)`,
			character, reason, notation, difficulty, rollsJSON, successes,
		)
		if err != nil {
			log.Print("error saving roll: ", err)

		}
		line := formatRollLine(character, reason, successes)
		htmlLine := fmt.Sprintf(`<div class="roll-entry">%s</div>`, line)

		select {
		case hub.broadcast <- htmlLine:
		default:
			// nobody's listening right now, or the hub's momentarily busy —
			// don't let a broadcast stall the roller's own response
		}
	}
}

func recentRolls(limit int) ([]string, error) {
	rows, err := db.Query(
		`SELECT character, reason, successes, rolled_at FROM rolls ORDER BY rolled_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var character, reason string
		var successes int
		var rolledAt time.Time
		if err := rows.Scan(&character, &reason, &successes, &rolledAt); err != nil {
			return nil, err
		}
		lines = append(lines, formatRollLineAt(character, reason, successes, rolledAt))
	}
	return lines, rows.Err()
}

func formatRollLineAt(character, reason string, successes int, t time.Time) string {
	// same body as formatRollLine, but using t.Format(...) instead of time.Now().Format(...)
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

func formatRollLine(character, reason string, successes int) string {
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

	return fmt.Sprintf("%s rolled %s: %d %s (%s)",
		label, action, successes, successWord, time.Now().Format("03:04:05PM"))
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
