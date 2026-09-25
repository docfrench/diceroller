package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite" // blank import registers the driver with database/sql
)

var diceNotation = regexp.MustCompile(`^(\d+)d(\d+)([+-]\d+)?$`)

type RollResult struct {
	Display    string
	LogLine    string
	Rolls      []int
	RollType   string
	Modifier   int
	Total      int
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
			`INSERT INTO rolls (character, reason, roll_type, notation, difficulty, successes, rolls, total, modifier) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			character, reason, result.RollType, result.Notation, difficulty, successes, string(rollsJSON), result.Total, result.Modifier,
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
		`SELECT character, reason, notation, successes, rolled_at, roll_type, total, modifier, rolls FROM rolls ORDER BY rolled_at DESC LIMIT ?`,
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
		var rolls []byte

		var rollType, notation string
		var modifier, total sql.NullInt64
		if err := rows.Scan(&character, &reason, &notation, &successes, &rolledAt, &rollType, &total, &modifier, &rolls); err != nil {
			return nil, err
		}

		successCount := 0
		if successes.Valid {
			successCount = int(successes.Int64)
		}
		modifierValue := 0
		if modifier.Valid {
			modifierValue = int(modifier.Int64)
		}
		totalValue := 0
		if total.Valid {
			totalValue = int(total.Int64)
		}
		lines = append(lines, formatRollLineAt(character, reason, notation, successCount, rollType, rolledAt, modifierValue, totalValue, json.RawMessage(rolls)))
	}
	return lines, rows.Err()
}

func formatRollLineAt(character, reason, notation string, successes int, rollType string, t time.Time, modifier, total int, rolls json.RawMessage) string {
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
	switch rollType {
	case "d20":
		if modifier != 0 {
			action = fmt.Sprintf("%s (%s%+d)", action, notation, modifier)
		} else {
			action = fmt.Sprintf("%s (%s)", action, notation)
		}
		return fmt.Sprintf("%s rolled %s: %d (%s) [%s]",
			label, action, total, t.Format("03:04PM"), rollType)
	case "d100":
		action = fmt.Sprintf("%s (1d100)", action)
		var dicerolls []int

		if err := json.Unmarshal(rolls, &dicerolls); err != nil {
			log.Print("error unmarshaling rolls for log line: ", err)
		}

		rollValue := 0
		if len(dicerolls) > 0 {
			rollValue = dicerolls[0]
		}

		return fmt.Sprintf("%s rolled %s: %d (%s) [%s]",
			label, action, rollValue, t.Format("03:04PM"), rollType)
	case "storyteller":
		action = fmt.Sprintf("%s (%s)", action, rollType)
		return fmt.Sprintf("%s rolled %s: %d %s (%s) [%s]",
			label, action, successes, successWord, t.Format("03:04PM"), rollType)
	default:
		action = fmt.Sprintf("%s (%s)", action, rollType)
		return fmt.Sprintf("%s rolled %s: %d %s (%s) [%s]",
			label, action, successes, successWord, t.Format("03:04PM"), rollType)
	}

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
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	log.Fatal(http.ListenAndServe(":8080", nil))
}
