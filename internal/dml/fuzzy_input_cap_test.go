package dml

import (
	"strings"
	"testing"
	"time"
)

func TestFuzzyScoreBlankInputDoesNotMatch(t *testing.T) {
	for method := range fuzzyScorers {
		for _, pair := range [][2]string{{"", ""}, {" \t\n", "  "}, {"", "abc"}, {"abc", " "}} {
			for _, blanks := range []bool{false, true} {
				if ok, score := FuzzyScore(method, pair[0], pair[1], blanks); ok || score != 0 {
					t.Errorf("%s %q vs %q blanks=%v: got (%v, %v), want (false, 0)", method, pair[0], pair[1], blanks, ok, score)
				}
			}
		}
	}
}

func TestFuzzyScoreLongInputIsBounded(t *testing.T) {
	long := strings.Repeat("Main Street Apt 4 ", 6000) // about 108k characters
	other := strings.Repeat("Mian Stret Apt 4 ", 6000)
	for method := range fuzzyScorers {
		start := time.Now()
		FuzzyScore(method, long, other, false)
		FuzzyScore(method, long, long, true)
		if d := time.Since(start); d > 50*time.Millisecond {
			t.Errorf("%s took %v on 100k-character input, want under 50ms", method, d)
		}
	}
}

func TestFuzzyScoreCapKeepsFieldSizedInputsExact(t *testing.T) {
	value := strings.Repeat("a", 255)
	if ok, _ := FuzzyScore("street", "1 "+value+" St", "1 "+value+" St", false); !ok {
		t.Fatal("identical 255-character street values must match")
	}
}
