package apextest

import (
	"sync"
	"testing"
)

func TestClassDispatcherDispatchesLargestFirst(t *testing.T) {
	costs := map[string]int64{"a": 100, "b": 500, "c": 250}
	order := []string{"a", "b", "c"}
	d := newClassDispatcher(order, costs, nil)
	got := []string{}
	for i := 0; i < 3; i++ {
		name, ok := d.next()
		if !ok {
			t.Fatalf("dispatcher drained early at %d", i)
		}
		d.recordObserved(name, 1)
		got = append(got, name)
	}
	want := []string{"b", "c", "a"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dispatch order = %v, want %v", got, want)
		}
	}
}

func TestClassDispatcherPrefersObservedDuration(t *testing.T) {
	costs := map[string]int64{"a": 1000, "b": 100}
	durations := map[string]int64{"a": 1, "b": 999}
	d := newClassDispatcher([]string{"a", "b"}, costs, durations)
	name, _ := d.next()
	if name != "b" {
		t.Fatalf("expected observed-duration override to dispatch b first, got %s", name)
	}
}

func TestAdaptiveMethodBudgetGivesDominantClassMoreWorkers(t *testing.T) {
	classes := []classScheduleInput{
		{ClassName: "BigClass", Methods: 40, DurationMS: 400_000},
		{ClassName: "SmallA", Methods: 2, DurationMS: 1_000},
		{ClassName: "SmallB", Methods: 2, DurationMS: 1_000},
		{ClassName: "SmallC", Methods: 2, DurationMS: 1_000},
	}

	budget := adaptiveClassMethodBudget(4, classes)

	if budget["BigClass"] < 2 {
		t.Fatalf("BigClass budget = %d, want at least 2", budget["BigClass"])
	}
}

func TestClassDispatcherStableTieBreak(t *testing.T) {
	d := newClassDispatcher([]string{"a", "b", "c"}, nil, nil)
	got := []string{}
	for i := 0; i < 3; i++ {
		name, _ := d.next()
		d.recordObserved(name, 0)
		got = append(got, name)
	}
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stable tiebreak failed: got %v want %v", got, want)
		}
	}
}

func TestClassDispatcherConcurrentDrain(t *testing.T) {
	const n = 200
	order := make([]string, n)
	costs := make(map[string]int64, n)
	for i := 0; i < n; i++ {
		name := string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+i%7))
		// ensure uniqueness via numeric suffix
		name = name + "-" + itoa(i)
		order[i] = name
		costs[name] = int64(i)
	}
	d := newClassDispatcher(order, costs, nil)
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]bool{}
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				name, ok := d.next()
				if !ok {
					return
				}
				mu.Lock()
				if seen[name] {
					mu.Unlock()
					t.Errorf("duplicate dispatch: %s", name)
					return
				}
				seen[name] = true
				mu.Unlock()
				d.recordObserved(name, 1)
			}
		}()
	}
	wg.Wait()
	if len(seen) != n {
		t.Fatalf("dispatched %d, want %d", len(seen), n)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
