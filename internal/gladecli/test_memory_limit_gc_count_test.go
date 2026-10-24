package gladecli

import (
	"runtime"
	"runtime/debug"
	"testing"
)

func TestApplyTestMemoryLimitsAllocationCollectionCountRoute(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "2GiB")
	oldLimit := debug.SetMemoryLimit(2 << 30)
	oldGC := debug.SetGCPercent(100)
	defer debug.SetMemoryLimit(oldLimit)
	defer debug.SetGCPercent(oldGC)
	applyTestMemoryLimits(true)
	// Retain 32 MiB and churn 128 MiB, comfortably below the memory ceiling.
	// Heap-growth collections here add work without relieving memory pressure.
	const chunk = 32 << 10
	live := make([][]byte, 1024)
	for i := range live {
		live[i] = make([]byte, chunk)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := range 4096 {
		data := make([]byte, chunk)
		data[0] = byte(i)
		live[i%len(live)] = data
	}
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(live)
	collections := after.NumGC - before.NumGC
	t.Logf("128 MiB allocation churn: collections=%d allocated=%d", collections, after.TotalAlloc-before.TotalAlloc)
	if collections > 1 {
		t.Fatalf("below-limit allocation churn triggered %d collections; want <=1", collections)
	}
}

func TestApplyTestMemoryLimitsPreservesExplicitPolicyOnReentryRoute(t *testing.T) {
	t.Setenv("GOMEMLIMIT", "1GiB")
	oldLimit := debug.SetMemoryLimit(1 << 30)
	oldGC := debug.SetGCPercent(100)
	defer debug.SetMemoryLimit(oldLimit)
	defer debug.SetGCPercent(oldGC)
	applyTestMemoryLimits(true)
	// Reentry without the flag must retain the policy already chosen by the
	// process owner, including its explicit soft ceiling.
	applyTestMemoryLimits(false)
	if got := debug.SetGCPercent(-1); got != -1 {
		t.Errorf("reentry GC percent = %d, want -1", got)
	}
	if got := debug.SetMemoryLimit(-1); got != 1<<30 {
		t.Errorf("reentry memory limit = %d, want 1 GiB", got)
	}
}
