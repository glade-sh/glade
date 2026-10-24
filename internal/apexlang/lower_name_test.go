package apexlang

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestLowerNameMatchesToLower(t *testing.T) {
	memo := lowerNameMemo{limit: 32}
	for _, input := range []string{"", "Account", "account", " Account ", "Straße", "Σςσ", "İIıi", "KKk", "ſSs", "A\xffZ", "\xff", "\u2003Account\u00a0", strings.Repeat("A", lowerNameMaxBytes+1), strings.Repeat("\xff", lowerNameMaxBytes)} {
		for range 2 {
			if got, want := memo.lower(input), strings.ToLower(input); got != want {
				t.Errorf("lower(%q) = %q, want %q", input, got, want)
			}
		}
	}
	// Lowercasing and simple folding have different equivalence classes.
	if memo.lower("ſ") == memo.lower("S") || memo.lower("ς") == memo.lower("Σ") {
		t.Fatal("lookup keys changed from lowercase to simple case folding")
	}
}

func TestLowerNameMemoBounds(t *testing.T) {
	memo := lowerNameMemo{limit: 2}
	for _, input := range []string{strings.Repeat("A", lowerNameMaxBytes+1), strings.Repeat("\xff", lowerNameMaxBytes), "already_lower"} {
		if got, want := memo.lower(input), strings.ToLower(input); got != want {
			t.Fatal("uncached result changed")
		}
	}
	if memo.count != 0 {
		t.Fatal("unchanged or oversized name retained")
	}
	for _, input := range []string{"FirstName", "SecondName", "ThirdName"} {
		if got, want := memo.lower(input), strings.ToLower(input); got != want {
			t.Fatal("bounded result changed")
		}
	}
	if memo.count != 2 {
		t.Fatalf("retained %d names, want 2", memo.count)
	}
	if got := testing.AllocsPerRun(100, func() { memo.lower("FirstName") }); got != 0 {
		t.Fatalf("full memo lost warm entry: %.0f allocations", got)
	}
}

func TestLowerNameConcurrentMemoBounds(t *testing.T) {
	memo := lowerNameMemo{limit: 32}
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			for i := range 100 {
				name := fmt.Sprintf("Field%d_%d", worker, i)
				if got, want := memo.lower(name), strings.ToLower(name); got != want {
					t.Errorf("concurrent lower(%q) = %q, want %q", name, got, want)
				}
			}
		})
	}
	workers.Wait()
	count := 0
	memo.names.Range(func(_, _ any) bool { count++; return true })
	if count != memo.limit || memo.count != memo.limit {
		t.Fatalf("retained %d names, counted %d; limit %d", count, memo.count, memo.limit)
	}
}
