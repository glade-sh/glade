package browserapi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelism(t *testing.T) {
	for _, tc := range []struct {
		name, request string
		memory        uint64
		passes, want  int
	}{
		{"default", "", MinimumMemory, 3, 1},
		{"disabled", "1", MinimumMemory, 3, 1},
		{"pair", "2", MinimumMemory, 3, 2},
		{"ceiling", "8", MinimumMemory, 5, 2},
		{"one-pass", "2", MinimumMemory, 1, 1},
		{"eight-gib", "2", 8 << 30, 3, 1},
		{"below-budget", "2", MinimumMemory - 1, 3, 1},
		{"unknown-memory", "2", 0, 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parallelism(tc.passes, tc.request, tc.memory); got != tc.want {
				t.Fatalf("parallelism = %d, want %d", got, tc.want)
			}
		})
	}
	for _, value := range []string{"0", "-1", "x"} {
		t.Run(value, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("invalid count did not panic")
				}
			}()
			parallelism(2, value, MinimumMemory)
		})
	}
}

func TestRun(t *testing.T) {
	// Configure GLADE_HOME externally when running the concurrent cases.
	for _, parallel := range []int{1, 2, 8} {
		t.Run(fmt.Sprint(parallel), func(t *testing.T) {
			var active, peak, cleaned atomic.Int32
			got := Run(t, []string{"59.0", "62.0", "67.0", "68.0"}, parallel, func(t *testing.T, pass int) int {
				t.Cleanup(func() { cleaned.Add(1) })
				n := active.Add(1)
				for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
				}
				defer active.Add(-1)
				time.Sleep(time.Duration(4-pass) * 10 * time.Millisecond)
				return pass * pass
			})
			if !reflect.DeepEqual(got, []int{0, 1, 4, 9}) {
				t.Fatalf("results = %v", got)
			}
			if peak.Load() > int32(min(parallel, 2)) {
				t.Fatalf("peak = %d", peak.Load())
			}
			if parallel > 1 && cleaned.Load() != 4 {
				t.Fatalf("cleanup incomplete: %d", cleaned.Load())
			}
		})
	}
}

func TestPrepareDefaultIsLazy(t *testing.T) {
	t.Setenv("GLADE_LWC_BROWSER_API_PARALLEL", "")
	calls := 0
	observe := Prepare(t, []string{"59.0", "67.0"}, func(t *testing.T, api string) string { calls++; return api })
	if calls != 0 {
		t.Fatalf("default eagerly observed %d APIs", calls)
	}
	for index, api := range []string{"59.0", "67.0"} {
		if got := observe(api); got != api || calls != index+1 {
			t.Fatalf("API %s = %s, calls %d", api, got, calls)
		}
	}
}

func TestRunPropagatesFatal(t *testing.T) {
	if os.Getenv("GLADE_BROWSER_API_FATAL_HELPER") == "1" {
		Run(t, []string{"59.0", "67.0"}, 2, func(t *testing.T, pass int) int { t.Fatal("synthetic browser failure"); return pass })
		t.Fatal("scheduler returned after child failure")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunPropagatesFatal$", "-test.parallel=2")
	cmd.Env = append(os.Environ(), "GLADE_BROWSER_API_FATAL_HELPER=1")
	output, err := cmd.CombinedOutput()
	if err == nil || ctx.Err() != nil || !bytes.Contains(output, []byte("synthetic browser failure")) || bytes.Contains(output, []byte("scheduler returned")) {
		t.Fatalf("child failure was not propagated cleanly: %v: %s", err, output)
	}
}
