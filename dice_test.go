package main

import (
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

func formRequest(values url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/roll", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// ---- rollD20 ----

func TestRollD20_Standard(t *testing.T) {
	for i := 0; i < 100; i++ {
		r := formRequest(url.Values{
			"d20_notation": {"1d20"},
			"modifier":     {"3"},
			"d20_adv":      {"standard"},
		})
		res, err := rollD20(r, "Alice", "attack")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.RollType != "d20" {
			t.Errorf("expected RollType d20, got %s", res.RollType)
		}
		if res.Total < 1+3 || res.Total > 20+3 {
			t.Errorf("total %d out of range [4,23]", res.Total)
		}
		if res.Modifier != 3 {
			t.Errorf("expected modifier 3, got %d", res.Modifier)
		}
	}
}

func TestRollD20_Advantage_KeepsHigher(t *testing.T) {
	for i := 0; i < 200; i++ {
		r := formRequest(url.Values{
			"d20_notation": {"1d20"},
			"modifier":     {"0"},
			"d20_adv":      {"advantage"},
		})
		res, err := rollD20(r, "Bob", "save")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Rolls) != 2 {
			t.Fatalf("expected 2 rolls recorded, got %d", len(res.Rolls))
		}
		if res.Rolls[0] < res.Rolls[1] {
			t.Errorf("advantage should keep the higher roll: got %v", res.Rolls)
		}
		if res.Total != res.Rolls[0] {
			t.Errorf("total should equal kept roll + modifier(0): total=%d rolls=%v", res.Total, res.Rolls)
		}
	}
}

func TestRollD20_Disadvantage_KeepsLower(t *testing.T) {
	for i := 0; i < 200; i++ {
		r := formRequest(url.Values{
			"d20_notation": {"1d20"},
			"modifier":     {"0"},
			"d20_adv":      {"disadvantage"},
		})
		res, err := rollD20(r, "Bob", "save")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Rolls[0] > res.Rolls[1] {
			t.Errorf("disadvantage should keep the lower roll: got %v", res.Rolls)
		}
	}
}

func TestRollD20_NonD20Sides(t *testing.T) {
	// sides != 20 goes through the generic multi-dice branch
	r := formRequest(url.Values{
		"d20_notation": {"3d6"},
		"modifier":     {"2"},
		"d20_adv":      {"standard"},
	})
	res, err := rollD20(r, "Carol", "damage")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Total < 3*1+2 || res.Total > 3*6+2 {
		t.Errorf("total %d out of expected range [5,20]", res.Total)
	}
}

func TestRollD20_InvalidNotation(t *testing.T) {
	r := formRequest(url.Values{
		"d20_notation": {"not-dice"},
		"modifier":     {"0"},
		"d20_adv":      {"standard"},
	})
	_, err := rollD20(r, "Dave", "test")
	if err == nil {
		t.Fatal("expected error for invalid notation, got nil")
	}
}

func TestRollD20_InvalidModifierDefaultsToZero(t *testing.T) {
	r := formRequest(url.Values{
		"d20_notation": {"1d20"},
		"modifier":     {"not-a-number"},
		"d20_adv":      {"standard"},
	})
	res, err := rollD20(r, "Eve", "test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Modifier != 0 {
		t.Errorf("expected modifier to default to 0, got %d", res.Modifier)
	}
	if res.Total != res.Rolls[0] {
		t.Errorf("with modifier 0, total should equal the roll: total=%d roll=%d", res.Total, res.Rolls[0])
	}
}

// ---- rollD100 ----

func expectedDeltaGOutcome(roll, target int) string {
	rollstr := strconv.Itoa(roll)
	isDouble := roll > 9 && rollstr[0] == rollstr[1]
	if roll <= target {
		if isDouble {
			return "Critical Success"
		}
		return "Success"
	}
	if isDouble {
		return "Critical Failure"
	}
	return "Failure"
}

func expectedCthulhuOutcome(roll, target int) string {
	hard := int(math.Floor(float64(target) / 2))
	extreme := int(math.Floor(float64(target) / 5))
	if roll <= target {
		if roll < extreme {
			return "Extreme Success"
		}
		if roll < hard {
			return "Hard Success"
		}
		return "Success"
	}
	return "Failure"
}

func TestRollD100_DeltaGreen_Consistency(t *testing.T) {
	for target := 1; target <= 100; target += 7 {
		for i := 0; i < 20; i++ {
			r := formRequest(url.Values{
				"target_pct": {strconv.Itoa(target)},
				"d100_rul":   {"delta_g"},
			})
			res, err := rollD100(r, "Frank", "sanity check")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			roll := res.Rolls[0]
			want := expectedDeltaGOutcome(roll, target)
			if !strings.Contains(res.LogLine, want) {
				t.Errorf("target=%d roll=%d: expected outcome %q in log line, got %q", target, roll, want, res.LogLine)
			}
		}
	}
}

func TestRollD100_Cthulhu_Consistency(t *testing.T) {
	for target := 1; target <= 100; target += 7 {
		for i := 0; i < 20; i++ {
			r := formRequest(url.Values{
				"target_pct": {strconv.Itoa(target)},
				"d100_rul":   {"cthulhu"},
			})
			res, err := rollD100(r, "Grace", "spot hidden")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			roll := res.Rolls[0]
			want := expectedCthulhuOutcome(roll, target)
			if !strings.Contains(res.LogLine, want) {
				t.Errorf("target=%d roll=%d: expected outcome %q in log line, got %q", target, roll, want, res.LogLine)
			}
		}
	}
}

func TestRollD100_InvalidTarget(t *testing.T) {
	cases := []string{"0", "101", "abc", ""}
	for _, target := range cases {
		r := formRequest(url.Values{
			"target_pct": {target},
			"d100_rul":   {"cthulhu"},
		})
		_, err := rollD100(r, "Henry", "test")
		if err == nil {
			t.Errorf("expected error for target %q, got nil", target)
		}
	}
}

func TestRollD100_InvalidRuleset(t *testing.T) {
	r := formRequest(url.Values{
		"target_pct": {"50"},
		"d100_rul":   {"not-a-real-system"},
	})
	_, err := rollD100(r, "Ivy", "test")
	if err == nil {
		t.Fatal("expected error for invalid ruleset, got nil")
	}
}

// ---- rollStoryteller ----

func TestRollStoryteller_SuccessCountConsistency(t *testing.T) {
	for i := 0; i < 200; i++ {
		r := formRequest(url.Values{
			"st_notation": {"6d10"},
			"difficulty":  {"6"},
		})
		res, err := rollStoryteller(r, "Jules", "resist frenzy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := 0
		for _, roll := range res.Rolls {
			if roll >= 6 {
				expected++
			}
			if roll == 1 {
				expected--
			}
		}
		if res.Successes == nil {
			t.Fatal("expected Successes to be set")
		}
		if *res.Successes != expected {
			t.Errorf("rolls=%v difficulty=6: expected %d successes, got %d", res.Rolls, expected, *res.Successes)
		}
		if res.Difficulty == nil || *res.Difficulty != 6 {
			t.Errorf("expected difficulty 6 recorded, got %v", res.Difficulty)
		}
		if len(res.Rolls) != 6 {
			t.Errorf("expected 6 dice rolled, got %d", len(res.Rolls))
		}
	}
}

func TestRollStoryteller_InvalidDifficulty(t *testing.T) {
	cases := []string{"1", "11", "abc", ""}
	for _, difficulty := range cases {
		r := formRequest(url.Values{
			"st_notation": {"5d10"},
			"difficulty":  {difficulty},
		})
		_, err := rollStoryteller(r, "Kara", "test")
		if err == nil {
			t.Errorf("expected error for difficulty %q, got nil", difficulty)
		}
	}
}

func TestRollStoryteller_InvalidNotation(t *testing.T) {
	r := formRequest(url.Values{
		"st_notation": {"banana"},
		"difficulty":  {"6"},
	})
	_, err := rollStoryteller(r, "Liam", "test")
	if err == nil {
		t.Fatal("expected error for invalid notation, got nil")
	}
}
