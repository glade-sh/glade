package gladecli

import (
	"context"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
	"time"
)

func TestGCFloorPercentHoldsGoalAtFloorThenFollowsGrowth(t *testing.T) {
	const floor = 512 << 20
	for _, tc := range []struct {
		name  string
		live  uint64
		roots uint64
	}{
		{"below runtime minimum", 1 << 20, 10 << 20},
		{"small heap", 6 << 20, 10 << 20},
		{"just under half", floor/2 - 1, 10 << 20},
	} {
		got := gcFloorPercent(floor, tc.live, tc.roots)
		live := max(tc.live, 4<<20)
		goal := live + (live+tc.roots)*uint64(got)/100
		if goal < floor || goal > floor+floor/50 {
			t.Errorf("%s: goal %d is not within 2%% above the floor %d", tc.name, goal, uint64(floor))
		}
	}
	for _, live := range []uint64{floor / 2, 3 << 30} {
		if got := gcFloorPercent(floor, live, 10<<20); got != 100 {
			t.Errorf("live %d: percent = %d, want 100", live, got)
		}
	}
}

func TestGCFloorForHostScalesWithMemoryBudgetAndLimit(t *testing.T) {
	const mib = 1 << 20
	const gib = 1 << 30
	for _, tc := range []struct {
		name   string
		budget uint64
		limit  int64
		want   uint64
	}{
		{"large host", 64 * gib, math.MaxInt64, 256 * mib},
		{"4 GiB host", 4 * gib, math.MaxInt64, 256 * mib},
		{"2 GiB container", 2 * gib, math.MaxInt64, 256 * mib},
		{"1 GiB container", 1 * gib, math.MaxInt64, 128 * mib},
		{"512 MiB container", 512 * mib, math.MaxInt64, 64 * mib},
		{"256 MiB container is off", 256 * mib, math.MaxInt64, 0},
		{"unknown budget uses conservative fallback", 0, math.MaxInt64, 256 * mib},
		{"explicit soft limit caps floor", 64 * gib, 1 * gib, 256 * mib},
		{"tiny soft limit is off", 64 * gib, 128 * mib, 0},
		{"unlimited sentinel ignored", 64 * gib, -1, 256 * mib},
	} {
		if got := gcFloorForHost(testGCFloorBytes, tc.budget, tc.limit); got != tc.want {
			t.Errorf("%s: floor = %d MiB, want %d MiB", tc.name, got/mib, tc.want/mib)
		}
	}
}

func TestParseCgroupLimitAndMeminfo(t *testing.T) {
	for text, want := range map[string]uint64{
		"max\n": 0, "": 0, "garbage": 0, "9223372036854771712\n": 0,
		"536870912\n": 536870912, "2147483648": 2147483648,
	} {
		if got := parseCgroupLimit(text); got != want {
			t.Errorf("parseCgroupLimit(%q) = %d, want %d", text, got, want)
		}
	}
	if got := parseMeminfoTotal("MemFree: 1 kB\nMemTotal:       1048576 kB\n"); got != 1<<30 {
		t.Errorf("parseMeminfoTotal = %d, want 1 GiB", got)
	}
	if got := parseMeminfoTotal("nothing"); got != 0 {
		t.Errorf("parseMeminfoTotal(no total) = %d, want 0", got)
	}
}

func TestHostMemoryBudgetIsReadable(t *testing.T) {
	if budget := hostMemoryBudget(); budget != 0 && budget < 64<<20 {
		t.Fatalf("hostMemoryBudget = %d, implausibly small", budget)
	}
}

func TestEnableTestGCFloorOnlyForTestCommandWithoutGOGC(t *testing.T) {
	defer func() { testGCFloorEnabled = false }()
	for _, args := range [][]string{nil, {"exec"}, {"parse"}, {"serve"}} {
		testGCFloorEnabled = false
		EnableTestGCFloor(args)
		if testGCFloorEnabled {
			t.Errorf("EnableTestGCFloor(%v) enabled the floor", args)
		}
	}
	t.Setenv("GOGC", "150")
	testGCFloorEnabled = false
	EnableTestGCFloor([]string{"test"})
	if testGCFloorEnabled {
		t.Error("explicit GOGC must keep the floor off")
	}
	t.Setenv("GOGC", "")
	EnableTestGCFloor([]string{"test"})
	if !testGCFloorEnabled {
		t.Error("plain test command should enable the floor")
	}
}

func TestGCFloorRouteEligibleExcludesServerDaemonWatchDebugConnect(t *testing.T) {
	if !gcFloorRouteEligible(false, false, false, false) {
		t.Error("a standalone local run must be eligible")
	}
	for i, flags := range [][4]bool{{true}, {false, true}, {false, false, true}, {false, false, false, true}} {
		if gcFloorRouteEligible(flags[0], flags[1], flags[2], flags[3]) {
			t.Errorf("route %d (daemon, watch, debug, connect) must not be eligible", i)
		}
	}
}

func TestServerAndControlRoutesNeverStartTheFloor(t *testing.T) {
	old := debug.SetGCPercent(73)
	defer debug.SetGCPercent(old)
	defer func() { testGCFloorEnabled = false }()
	for _, args := range [][]string{{"--help"}, {"clear-cache", "--help"}} {
		testGCFloorEnabled = true
		_, _ = runTest(context.Background(), args, io.Discard, io.Discard)
		if !testGCFloorEnabled {
			t.Errorf("runTest(%v) consumed the floor on a control route", args)
		}
	}
	if got := debug.SetGCPercent(73); got != 73 {
		t.Fatalf("GC percent = %d, want untouched 73", got)
	}
}

func TestStartTestGCFloorIfEnabledIsInertWhenNotEnabled(t *testing.T) {
	old := debug.SetGCPercent(73)
	defer debug.SetGCPercent(old)
	testGCFloorEnabled = false
	startTestGCFloorIfEnabled()
	if got := debug.SetGCPercent(73); got != 73 {
		t.Fatalf("GC percent = %d, want untouched 73", got)
	}
}

func TestGCFloorTunerStandsDownWhenAnotherPolicyChangesPercent(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(-1)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	tuner := newGCFloorTuner(1 << 40)
	if tuner == nil {
		t.Fatal("runtime metrics unavailable")
	}
	// Another policy takes the percent, then the live heap passes the floor
	// so the tuner has a new value to write.
	debug.SetGCPercent(-1)
	tuner.floor = 0
	if !tuner.step() {
		t.Fatal("tuner stayed armed after another policy took over")
	}
	if got := debug.SetGCPercent(-1); got != -1 {
		t.Fatalf("GC percent = %d, want the other policy's -1", got)
	}
}

func TestGCFloorBackstopLimitIsInstalledAndRestored(t *testing.T) {
	// Other CLI tests retain more than half the production floor. This test
	// exercises the backstop, so keep its floor well above the suite's heap.
	const floor = 1 << 40
	const priorLimit = 4 * floor
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(priorLimit)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	tuner := newGCFloorTuner(floor)
	if tuner == nil {
		t.Fatal("tuner did not start below the synthetic backstop test floor")
	}
	if got := debug.SetMemoryLimit(-1); got != 2*floor {
		t.Fatalf("limit while floor is in force = %d, want %d", got, int64(2*floor))
	}
	tuner.release()
	if got := debug.SetMemoryLimit(-1); got != priorLimit {
		t.Fatalf("limit after floor = %d, want the prior %d", got, int64(priorLimit))
	}
}

func TestGCFloorBackstopKeepsLowerExplicitLimit(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(300 << 20)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	// Match the large synthetic floor in the backstop installation test.
	tuner := newGCFloorTuner(1 << 40)
	if tuner == nil {
		t.Fatal("tuner did not start below the synthetic backstop test floor")
	}
	if got := debug.SetMemoryLimit(-1); got != 300<<20 {
		t.Fatalf("explicit lower limit changed to %d", got)
	}
	tuner.release()
	if got := debug.SetMemoryLimit(-1); got != 300<<20 {
		t.Fatalf("explicit limit not preserved: %d", got)
	}
}

func TestGCFloorDoesNotOverrideAnExplicitPercent(t *testing.T) {
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)
	if tuner := newGCFloorTuner(256 << 20); tuner != nil {
		t.Fatal("tuner started over an explicit GC percent")
	}
	if got := debug.SetGCPercent(-1); got != -1 {
		t.Fatalf("GC percent = %d, want -1 kept", got)
	}
}

func writeCgroupFile(t *testing.T, dir, name, value string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCgroupEffectiveLimitWalksAncestorsV2(t *testing.T) {
	root := t.TempDir()
	writeCgroupFile(t, root, "memory.max", "max\n")
	writeCgroupFile(t, filepath.Join(root, "user.slice"), "memory.max", "268435456\n")
	writeCgroupFile(t, filepath.Join(root, "user.slice", "app.scope"), "memory.max", "max\n")
	writeCgroupFile(t, filepath.Join(root, "user.slice", "app.scope", "leaf"), "memory.max", "1073741824\n")
	got := cgroupEffectiveLimit(root, "0::/user.slice/app.scope/leaf\n")
	if got != 268435456 {
		t.Fatalf("effective limit = %d, want the ancestor's 256 MiB", got)
	}
}

func TestCgroupEffectiveLimitWalksAncestorsV1(t *testing.T) {
	root := t.TempDir()
	mem := filepath.Join(root, "memory")
	writeCgroupFile(t, mem, "memory.limit_in_bytes", "9223372036854771712\n")
	writeCgroupFile(t, filepath.Join(mem, "docker"), "memory.limit_in_bytes", "536870912\n")
	writeCgroupFile(t, filepath.Join(mem, "docker", "c1"), "memory.limit_in_bytes", "1073741824\n")
	self := "12:cpu,cpuacct:/docker/c1\n4:memory:/docker/c1\n"
	if got := cgroupEffectiveLimit(root, self); got != 536870912 {
		t.Fatalf("effective limit = %d, want the ancestor's 512 MiB", got)
	}
}

func TestCgroupEffectiveLimitNamespacedAndUnlimitedViews(t *testing.T) {
	root := t.TempDir()
	// The process path does not exist under the mount: walk up to the root file.
	writeCgroupFile(t, root, "memory.max", "134217728\n")
	if got := cgroupEffectiveLimit(root, "0::/../../missing/child\n"); got != 134217728 {
		t.Fatalf("namespaced limit = %d, want 128 MiB", got)
	}
	if got := cgroupEffectiveLimit(t.TempDir(), "0::/x\n"); got != 0 {
		t.Fatalf("no readable files gave limit %d", got)
	}
	if got := cgroupEffectiveLimit(root, ""); got != 0 {
		t.Fatalf("empty self cgroup gave limit %d", got)
	}
}

func TestGCFloorNotStartedWhenLiveHeapIsAlreadyPastHalfTheFloor(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(8 << 30)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	// A 1 MiB floor is below twice the 4 MiB runtime minimum, so the policy is already 100.
	if tuner := newGCFloorTuner(1 << 20); tuner != nil {
		t.Fatal("tuner started with nothing to hold back")
	}
	if got := debug.SetMemoryLimit(-1); got != 8<<30 {
		t.Fatalf("limit changed to %d", got)
	}
}

func TestGCFloorGoroutineEndsAtMaxAgeAndRestoresPolicy(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(8 << 30)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	// A floor far above the live heap never reaches the hand-over, so only maxAge ends it.
	tuner := newGCFloorTuner(1 << 40)
	if tuner == nil {
		t.Fatal("tuner did not start")
	}
	done := make(chan struct{})
	go func() {
		tuner.run(time.Millisecond, 40*time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tuner goroutine did not end at maxAge")
	}
	if got := debug.SetGCPercent(100); got != 100 {
		t.Fatalf("GC percent after maxAge = %d, want 100", got)
	}
	if got := debug.SetMemoryLimit(-1); got != 8<<30 {
		t.Fatalf("limit after maxAge = %d, want the prior 8 GiB", got)
	}
}

func TestGCFloorGoroutineEndsAtHandOver(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(8 << 30)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	tuner := newGCFloorTuner(1 << 40)
	if tuner == nil {
		t.Fatal("tuner did not start")
	}
	tuner.floor = 0 // the live heap now counts as past half the floor
	done := make(chan struct{})
	go func() {
		tuner.run(time.Millisecond, time.Hour)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tuner goroutine did not end at hand-over")
	}
	if got := debug.SetGCPercent(100); got != 100 {
		t.Fatalf("GC percent after hand-over = %d, want 100", got)
	}
	if got := debug.SetMemoryLimit(-1); got != 8<<30 {
		t.Fatalf("limit after hand-over = %d, want the prior 8 GiB", got)
	}
}
