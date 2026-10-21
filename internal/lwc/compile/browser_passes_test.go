package compile_test

import (
	"os"
	"strconv"
	"sync"
	"testing"
)

// browserAPIParallelism is how many API passes of a browser conformance test
// run at once. GLADE_LWC_BROWSER_API_PARALLEL sets it explicitly; a host that
// caps row concurrency (GLADE_LWC_BROWSER_CONCURRENCY) runs the passes one at a
// time unless it says otherwise.
func browserAPIParallelism(passes int) int {
	if value := os.Getenv("GLADE_LWC_BROWSER_API_PARALLEL"); value != "" {
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 {
			panic("GLADE_LWC_BROWSER_API_PARALLEL must be a positive integer: " + value)
		}
		return min(count, passes)
	}
	if os.Getenv("GLADE_LWC_BROWSER_CONCURRENCY") != "" {
		return 1
	}
	return min(passes, 3)
}

// runBrowserPasses runs run(0..passes-1) with at most parallel in flight and
// returns the results in pass order. run must not call t.Fatal or t.FailNow:
// it executes off the test goroutine, so report failures through the result.
func runBrowserPasses[T any](passes, parallel int, run func(pass int) T) []T {
	results := make([]T, passes)
	slots := make(chan struct{}, max(1, parallel))
	var wait sync.WaitGroup
	for pass := range passes {
		wait.Add(1)
		slots <- struct{}{}
		go func() {
			defer wait.Done()
			defer func() { <-slots }()
			results[pass] = run(pass)
		}()
	}
	wait.Wait()
	return results
}

func TestRunBrowserPassesKeepsPassOrder(t *testing.T) {
	for _, parallel := range []int{1, 2, 5} {
		got := runBrowserPasses(5, parallel, func(pass int) int { return pass * pass })
		for pass, value := range got {
			if value != pass*pass {
				t.Fatalf("parallel %d pass %d = %d", parallel, pass, value)
			}
		}
	}
}

func TestBrowserAPIParallelism(t *testing.T) {
	t.Setenv("GLADE_LWC_BROWSER_API_PARALLEL", "")
	t.Setenv("GLADE_LWC_BROWSER_CONCURRENCY", "")
	if got := browserAPIParallelism(2); got != 2 {
		t.Fatalf("default for 2 passes = %d", got)
	}
	if got := browserAPIParallelism(5); got != 3 {
		t.Fatalf("default for 5 passes = %d", got)
	}
	t.Setenv("GLADE_LWC_BROWSER_CONCURRENCY", "3")
	if got := browserAPIParallelism(3); got != 1 {
		t.Fatalf("memory-tight host = %d", got)
	}
	t.Setenv("GLADE_LWC_BROWSER_API_PARALLEL", "2")
	if got := browserAPIParallelism(3); got != 2 {
		t.Fatalf("explicit = %d", got)
	}
}
