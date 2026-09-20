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

	_ "modernc.org/sqlite" // blank import registers the driver with database/sql
)

var db *sql.DB

var tablePassphrase = os.Getenv("TABLE_PASS")

var diceNotation = regexp.MustCompile(`^(\d+)d(\d+)([+-]\d+)?$`)

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

func RollHandler(w http.ResponseWriter, r *http.Request) {
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

}

func main() {
	if err := initDB(); err != nil {
		log.Fatal("failed to open database: ", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Print("error closing database: ", err)
		}
	}()

	fmt.Println("Dice Roller is listening on port 8080")
	fmt.Println("Enter passphrase: " + tablePassphrase)
	http.HandleFunc("/", HomePage)
	http.HandleFunc("/roll", RollHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
