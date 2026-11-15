package typesys

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// Measure a fresh view and its excluded-name index after their shared input
// tables are initialized. A subprocess isolates the heap from other tests.
func TestGatedPlatformViewRetainedHeap(t *testing.T) {
	if os.Getenv("GLADE_GATED_VIEW_HEAP_TEST") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedPlatformViewRetainedHeap$")
		cmd.Env = append(os.Environ(), "GLADE_GATED_VIEW_HEAP_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("heap subprocess: %v\n%s", err, output)
		}
		return
	}
	_ = gatedPlatformStubs()
	runtime.GC()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	view, excluded := buildGatedStandardPlatformSymbols(false, 0)
	runtime.GC()
	runtime.GC()
	runtime.ReadMemStats(&after)
	retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	runtime.KeepAlive(view)
	runtime.KeepAlive(excluded)
	if retained > 9<<20 {
		t.Fatalf("view and excluded-name index retain %d bytes, budget %d", retained, 9<<20)
	}
}
