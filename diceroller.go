package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math"
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

	notation := r.FormValue("d20_notation")
	modifier, err := strconv.Atoi(r.FormValue("modifier"))
	advantage := r.FormValue("d20_adv")
	note := ""

	var b strings.Builder
	var roll1, roll2, total int
	var logLine string

	matches := diceNotation.FindStringSubmatch(notation)
	if matches == nil {
		return RollResult{}, fmt.Errorf("invalid notation: %s", notation)
	}
	count, _ := strconv.Atoi(matches[1])
	sides, _ := strconv.Atoi(matches[2])

	if err != nil {
		modifier = 0
	}

	switch sides {
	case 20:
		switch advantage {
		case "advantage":
			roll1 = rand.Intn(20) + 1
			roll2 = rand.Intn(20) + 1
			if roll2 > roll1 {
				a := roll1
				roll1 = roll2
				roll2 = a
			}
			switch roll1 {
			case 20:
				note = `<span style="color:var(--good)"><br><br><strong>*~* Natural 20! *~*</strong></span>`
			case 1:
				note = `<span style="color:var(--oxblood-bright)"><br><br><strong>......Natural 1......</strong></span>`
			default:
				// no special note
			}
			total = roll1 + modifier
			fmt.Fprintf(&b, `<p><div align="center">%s rolled a <strong>d20</strong> with advantage%s<br><br>Rolls: %d and %d<br><br>Total: <strong>%d</strong></div></p>`, character, note, roll1, roll2, total)
		case "disadvantage":
			roll1 = rand.Intn(20) + 1
			roll2 = rand.Intn(20) + 1
			if roll2 < roll1 {
				a := roll1
				roll1 = roll2
				roll2 = a
			}
			switch roll1 {
			case 20:
				note = `<span style="color:var(--good)"><br><br><strong>*~* Natural 20! *~*</strong></span>`
			case 1:
				note = `<span style="color:var(--oxblood-bright)"><br><br><strong>......Natural 1......</strong></span>`
			default:
				// no special note
			}
			total = roll1 + modifier
			fmt.Fprintf(&b, `<p><div align="center">%s rolled a <strong>d20</strong> with disadvantage%s<br><br>Rolls: %d and %d<br><br>Total: <strong>%d</strong></div></p>`, character, note, roll1, roll2, total)
		default:
			roll1 = rand.Intn(20) + 1

			total = roll1 + modifier

			switch roll1 {
			case 20:
				note = `<span style="color:var(--good)"><br><br><strong>*~* Natural 20! *~*</strong></span>`
			case 1:
				note = `<span style="color:var(--oxblood-bright)"><br><br><strong>......Natural 1......</strong></span>`
			default:
				// no special note
			}

			fmt.Fprintf(&b, `<p><div align="center">%s rolled <strong>d20%+d</strong>%s<br><br>Roll: %d<br><br>Total: <strong>%d</strong></div></p>`,
				character, modifier, note, roll1, total)
		}
		logLine = fmt.Sprintf("%s rolled %s+%d to %s: %d + %d = %d total (%s)", character, notation, modifier, reason, roll1, modifier, total, time.Now().Format("03:04PM"))
	default:
		fmt.Fprintf(&b, `<p><div align="center">%s rolled <strong>%s+%d</strong> to %s<br><br>`,
			character, notation, modifier, reason)
		for i := 0; i < count; i++ {
			roll := rand.Intn(sides) + 1
			total += roll
			fmt.Fprintf(&b, `Roll: %d<br>`, roll)
		}
		total += modifier
		fmt.Fprintf(&b, `Total: <strong>%d</strong></div></p>`, total)
		logLine = fmt.Sprintf("%s rolled %s+%d to %s: %d total (%s)", character, notation, modifier, reason, total, time.Now().Format("03:04PM"))
	}

	return RollResult{
		Display:  b.String(),
		LogLine:  logLine,
		Notation: notation,
		Modifier: modifier,
		Total:    total,
		Rolls:    []int{roll1, roll2},
		RollType: "d20",
	}, nil

}

func rollD100(r *http.Request, character, reason string) (RollResult, error) {
	target, err := strconv.Atoi(r.FormValue("target_pct"))
	if err != nil || target < 1 || target > 100 {
		return RollResult{}, fmt.Errorf("invalid target: %s", r.FormValue("target_pct"))
	}
	var outcome string
	var b strings.Builder

	ruleset := r.FormValue("d100_rul")
	roll := rand.Intn(100) + 1
	rollstr := strconv.Itoa(roll)

	switch ruleset {
	case "delta_g":
		if roll <= target {
			fmt.Fprintf(&b, `<div align="center"><p>%s rolled <strong>d100</strong> vs target %d%%<br><br><span style="color:var(--good)">Roll: %d`,
				character, target, roll)
			if roll > 9 && rollstr[0] == rollstr[1] {
				fmt.Fprintf(&b, `<br><br>Critical Success</span></p></div>`)
			} else {
				fmt.Fprintf(&b, `<br><br>Success</span></p></div>`)
			}
		} else {
			fmt.Fprintf(&b, `<div align="center"><p>%s rolled <strong>d100</strong> vs target %d%%<br><br><span style="color:var(--oxblood-bright)">Roll: %d`,
				character, target, roll)
			if roll > 9 && rollstr[0] == rollstr[1] {
				fmt.Fprintf(&b, `<br><br>Critical Failure</span></p></div>`)
			} else {
				fmt.Fprintf(&b, `<br><br>Failure</span></p></div>`)
			}
		}
	case "cthulhu":
		hard := int(math.Floor(float64(target) / 2))
		extreme := int(math.Floor(float64(target) / 5))
		if roll <= target {
			fmt.Fprintf(&b, `<div align="center"><p>%s rolled <strong>d100</strong> vs target %d%%<br><br><span style="color:var(--good)">Roll: %d`,
				character, target, roll)
			if roll < extreme {
				fmt.Fprintf(&b, `<br><br>Extreme Success</span></p></div>`)
			} else if roll < hard {
				fmt.Fprintf(&b, `<br><br>Hard Success</span></p></div>`)
			} else {
				fmt.Fprintf(&b, `<br><br>Success</span></p></div>`)
			}
		} else {
			fmt.Fprintf(&b, `<div align="center"><p>%s rolled <strong>d100</strong> vs target %d%%<br><br><span style="color:var(--oxblood-bright)">Roll: %d<br><br>Failure</span></p></div>`,
				character, target, roll)

		}
	default:
		return RollResult{}, fmt.Errorf("invalid ruleset: %s", ruleset)
	}

	logLine := fmt.Sprintf("%s rolled %d against %d to %s: %s (%s)", character, roll, target, reason, outcome, time.Now().Format("03:04PM"))

	return RollResult{
		Display:  b.String(),
		LogLine:  logLine,
		Notation: r.FormValue("target_pct"),
		Rolls:    []int{roll},
		RollType: "d100",
	}, nil
}

func rollStoryteller(r *http.Request, character, reason string) (RollResult, error) {
	notation := r.FormValue("st_notation")
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
