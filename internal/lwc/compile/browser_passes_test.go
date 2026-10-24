package compile_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/testutil/browserapi"
)

// The shared scheduler also serves browser conformance outside this package.
func browserAPIParallelism(passes int) int { return browserapi.Parallelism(passes) }

func runBrowserPasses[T any](t *testing.T, versions []string, parallel int, run func(*testing.T, int) T) []T {
	t.Helper()
	return browserapi.Run(t, versions, parallel, run)
}

func TestRunBrowserPassesKeepsPassOrder(t *testing.T) {
	for _, parallel := range []int{1, 2, 5} {
		got := runBrowserPasses(t, []string{"0", "1", "2", "3", "4"}, parallel, func(t *testing.T, pass int) int { return pass * pass })
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
	if got := browserAPIParallelism(2); got != 1 {
		t.Fatalf("default for 2 passes = %d", got)
	}
	if got := browserAPIParallelism(5); got != 1 {
		t.Fatalf("default for 5 passes = %d", got)
	}
	t.Setenv("GLADE_LWC_BROWSER_CONCURRENCY", "3")
	if got := browserAPIParallelism(3); got != 1 {
		t.Fatalf("memory-tight host = %d", got)
	}
}
