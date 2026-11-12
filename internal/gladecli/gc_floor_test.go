package gladecli

import (
	"runtime/debug"
	"testing"
)

func TestGCFloorPercentHoldsGoalAtFloorThenFollowsGrowth(t *testing.T) {
	const floor = 512 << 20
	cases := []struct {
		name  string
		live  uint64
		roots uint64
		want  int
	}{
		{"below runtime minimum", 1 << 20, 10 << 20, int((uint64(floor)-4<<20)*100/(4<<20+10<<20)) + 1},
		{"small heap", 6 << 20, 10 << 20, int((uint64(floor)-6<<20)*100/(6<<20+10<<20)) + 1},
		{"just under half", floor/2 - 1, 10 << 20, int((uint64(floor)-(floor/2-1))*100/(floor/2-1+10<<20)) + 1},
		{"half the floor", floor / 2, 10 << 20, 100},
		{"large heap", 3 << 30, 10 << 20, 100},
	}
	for _, tc := range cases {
		got := gcFloorPercent(floor, tc.live, tc.roots)
		if got != tc.want {
			t.Errorf("%s: gcFloorPercent = %d, want %d", tc.name, got, tc.want)
		}
		if tc.want > 100 {
			live := max(tc.live, 4<<20)
			goal := live + (live+tc.roots)*uint64(got)/100
			if goal < floor || goal > floor+floor/50 {
				t.Errorf("%s: goal %d is not within 2%% above the floor %d", tc.name, goal, uint64(floor))
			}
		}
	}
}

func TestTuneTestGCLeavesOtherCommandsAndExplicitGOGC(t *testing.T) {
	old := debug.SetGCPercent(73)
	defer debug.SetGCPercent(old)
	TuneTestGC(nil)
	TuneTestGC([]string{"exec"})
	TuneTestGC([]string{"parse"})
	t.Setenv("GOGC", "150")
	TuneTestGC([]string{"test"})
	if got := debug.SetGCPercent(73); got != 73 {
		t.Fatalf("GC percent = %d, want it untouched at 73", got)
	}
}

func TestGCFloorTunerStandsDownWhenAnotherPolicyChangesPercent(t *testing.T) {
	old := debug.SetGCPercent(100)
	defer debug.SetGCPercent(old)
	tuner := &gcFloorTuner{floor: 512 << 20, last: 4321}
	tuner.sample = newGCFloorSamples()
	// The process percent is 100, not the 4321 the tuner last set: a caller such
	// as --gc-aggressive changed it. The tuner must disarm and keep that value.
	debug.SetGCPercent(-1)
	if tuner.afterCycle() {
		t.Fatal("tuner stayed armed after another policy took over")
	}
	if got := debug.SetGCPercent(-1); got != -1 {
		t.Fatalf("GC percent = %d, want the other policy's -1 restored", got)
	}
}

func TestGCFloorTunerFollowsLiveHeapWhileItOwnsThePercent(t *testing.T) {
	old := debug.SetGCPercent(100)
	defer debug.SetGCPercent(old)
	tuner := &gcFloorTuner{floor: 1 << 40, sample: newGCFloorSamples()}
	percent, ok := tuner.percent()
	if !ok {
		t.Fatal("runtime metrics unavailable")
	}
	tuner.last = percent
	debug.SetGCPercent(percent)
	if !tuner.afterCycle() {
		t.Fatal("tuner disarmed while it owned the percent")
	}
	if tuner.last <= 100 {
		t.Fatalf("tuner percent = %d, want above 100 for a floor far above the live heap", tuner.last)
	}
}
