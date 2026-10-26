// Package browserapi schedules isolated browser observations for conformance tests.
package browserapi

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
)

// MinimumMemory is the physical host budget for two browser API passes.
// Smaller hosts and hosts whose memory cannot be determined stay sequential.
const MinimumMemory uint64 = 16 << 30

// Parallelism defaults to one. An explicit positive API-pass count opts in,
// subject to the physical host budget and a hard ceiling of two browsers.
func Parallelism(passes int) int {
	request := os.Getenv("GLADE_LWC_BROWSER_API_PARALLEL")
	if request == "" || request == "1" {
		return min(passes, 1)
	}
	return parallelism(passes, request, physicalMemory())
}

func parallelism(passes int, request string, memory uint64) int {
	count := 1
	if request != "" {
		var err error
		count, err = strconv.Atoi(request)
		if err != nil || count < 1 {
			panic("GLADE_LWC_BROWSER_API_PARALLEL must be a positive integer: " + request)
		}
	}
	if memory < MinimumMemory {
		count = 1
	}
	return min(passes, count, 2)
}

func physicalMemory() uint64 {
	switch runtime.GOOS {
	case "darwin":
		data, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		value, _ := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		return value
	case "linux":
		data, err := os.ReadFile("/proc/meminfo")
		if err != nil {
			return 0
		}
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 3 && fields[0] == "MemTotal:" && fields[2] == "kB" {
				value, _ := strconv.ParseUint(fields[1], 10, 64)
				return value * 1024
			}
		}
	}
	return 0
}

// Run waits for every API subtest and returns results in the input order.
// Each callback receives its own testing.T, so Fatal, TempDir and Cleanup are
// safe. Pairwise groups enforce the ceiling even with a larger -parallel flag;
// Go's -parallel flag can reduce concurrency further.
func Run[T any](t *testing.T, versions []string, parallel int, run func(*testing.T, int) T) []T {
	t.Helper()
	results := make([]T, len(versions))
	parallel = min(max(1, parallel), 2)
	if parallel == 1 {
		for pass := range versions {
			results[pass] = run(t, pass)
		}
		return results
	}
	for start := 0; start < len(versions); start += parallel {
		if !t.Run(fmt.Sprintf("APIs%d", start/parallel), func(t *testing.T) {
			for pass := start; pass < min(start+parallel, len(versions)); pass++ {
				t.Run(versions[pass], func(t *testing.T) {
					t.Parallel()
					results[pass] = run(t, pass)
				})
			}
		}) {
			t.FailNow()
		}
	}
	return results
}

// Prepare observes eagerly only in opt-in mode. The default stays lazy to
// preserve each family's compile/observe/report sequence. Callers consume
// results in API order and keep counters and report writes on the parent.
func Prepare[T any](t *testing.T, versions []string, observe func(*testing.T, string) T) func(string) T {
	t.Helper()
	parallel := Parallelism(len(versions))
	if parallel <= 1 {
		return func(api string) T { return observe(t, api) }
	}
	if _, err := gladehome.EnsureRoot(); err != nil {
		t.Fatal(err)
	}
	results := Run(t, versions, parallel, func(t *testing.T, pass int) T { return observe(t, versions[pass]) })
	return func(api string) T {
		for pass, version := range versions {
			if api == version {
				return results[pass]
			}
		}
		t.Fatalf("unprepared browser API %s", api)
		var zero T
		return zero
	}
}
