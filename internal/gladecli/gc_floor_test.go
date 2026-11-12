package gladecli

import (
	"context"
	"io"
	"math"
	"runtime/debug"
	"testing"
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
		{"large host", 64 * gib, math.MaxInt64, 512 * mib},
		{"4 GiB host", 4 * gib, math.MaxInt64, 512 * mib},
		{"2 GiB container", 2 * gib, math.MaxInt64, 256 * mib},
		{"1 GiB container", 1 * gib, math.MaxInt64, 128 * mib},
		{"512 MiB container", 512 * mib, math.MaxInt64, 64 * mib},
		{"256 MiB container is off", 256 * mib, math.MaxInt64, 0},
		{"unknown budget uses conservative fallback", 0, math.MaxInt64, 256 * mib},
		{"explicit soft limit caps floor", 64 * gib, 1 * gib, 256 * mib},
		{"tiny soft limit is off", 64 * gib, 128 * mib, 0},
		{"unlimited sentinel ignored", 64 * gib, -1, 512 * mib},
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
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(8 << 30)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	const floor = 256 << 20
	tuner := newGCFloorTuner(floor)
	if tuner == nil {
		t.Fatal("runtime metrics unavailable")
	}
	if got := debug.SetMemoryLimit(-1); got != 2*floor {
		t.Fatalf("limit while floor is in force = %d, want %d", got, int64(2*floor))
	}
	tuner.finish(100)
	if got := debug.SetMemoryLimit(-1); got != 8<<30 {
		t.Fatalf("limit after floor = %d, want the prior 8 GiB", got)
	}
}

func TestGCFloorBackstopKeepsLowerExplicitLimit(t *testing.T) {
	oldGC := debug.SetGCPercent(100)
	oldLimit := debug.SetMemoryLimit(300 << 20)
	defer debug.SetGCPercent(oldGC)
	defer debug.SetMemoryLimit(oldLimit)
	tuner := newGCFloorTuner(256 << 20)
	if tuner == nil {
		t.Fatal("runtime metrics unavailable")
	}
	if got := debug.SetMemoryLimit(-1); got != 300<<20 {
		t.Fatalf("explicit lower limit changed to %d", got)
	}
	tuner.finish(100)
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
