package main

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite" // blank import registers the driver with database/sql
)

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
