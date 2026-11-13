package gladecli

import (
	"math"
	"os"
	"runtime/debug"
	"runtime/metrics"
	"time"
)

// testGCFloorBytes is the heap size below which a standalone `glade test`
// process does not start a collection. A cold test process builds its org and
// class catalog from nothing and the live heap climbs from a few MiB to several
// hundred. With the default collector target every doubling is a full mark of
// everything built so far. Holding the next-collection goal at this floor skips
// those cycles. Once the live heap passes half the floor, or after
// testGCFloorMaxAge, the target is the ordinary 100 percent growth.
const testGCFloorBytes = 256 << 20

const (
	// testGCFloorUnknownHostBytes is the floor when no host or container
	// memory budget can be read.
	testGCFloorUnknownHostBytes = 256 << 20
	// testGCFloorMinBytes is the smallest floor worth installing.
	testGCFloorMinBytes = 64 << 20
	// testGCFloorBudgetShare divides the memory budget: the floor never exceeds
	// one eighth of physical memory or of the container limit.
	testGCFloorBudgetShare = 8
	testGCFloorPollEvery   = 20 * time.Millisecond
	// testGCFloorMaxAge ends the tuner even if the live heap never reaches
	// half the floor: a cold start that is still small after this long does not
	// need the floor.
	testGCFloorMaxAge = 30 * time.Second
)

var testGCFloorEnabled bool

// EnableTestGCFloor marks the process as a standalone `glade test` candidate.
// It starts nothing: runTest installs the floor only on the route that runs
// tests in this process. An explicit GOGC always wins. Library callers and test
// binaries never call it and keep the runtime default.
func EnableTestGCFloor(args []string) {
	if len(args) == 0 || args[0] != "test" || os.Getenv("GOGC") != "" {
		return
	}
	testGCFloorEnabled = true
}

// gcFloorRouteEligible reports whether this runTest route executes the tests
// in the current process. Test servers (serve), daemons, watch loops, debug
// sessions and runs sent to a server are not eligible.
func gcFloorRouteEligible(daemonMode, watchMode, debugMode, connectMode bool) bool {
	return !daemonMode && !watchMode && !debugMode && !connectMode
}

func startTestGCFloorIfEnabled() {
	if !testGCFloorEnabled {
		return
	}
	testGCFloorEnabled = false
	floor := gcFloorForHost(testGCFloorBytes, hostMemoryBudget(), debug.SetMemoryLimit(-1))
	if floor == 0 {
		return
	}
	if tuner := newGCFloorTuner(floor); tuner != nil {
		go tuner.run(testGCFloorPollEvery, testGCFloorMaxAge)
	}
}

// gcFloorForHost caps the floor by the memory budget (physical memory or a
// container limit, 0 when unknown) and by a soft memory limit. A floor below
// the minimum is not worth the extra resident memory and returns 0 (off).
func gcFloorForHost(floor, budget uint64, softLimit int64) uint64 {
	if budget == 0 {
		floor = min(floor, testGCFloorUnknownHostBytes)
	} else {
		floor = min(floor, budget/testGCFloorBudgetShare)
	}
	if softLimit > 0 && softLimit != math.MaxInt64 {
		floor = min(floor, uint64(softLimit)/4)
	}
	if floor < testGCFloorMinBytes {
		return 0
	}
	return floor
}

type gcFloorTuner struct {
	floor      uint64
	last       int
	sample     []metrics.Sample
	priorLimit int64
	ownedLimit int64
}

func newGCFloorSamples() []metrics.Sample {
	return []metrics.Sample{
		{Name: "/gc/heap/live:bytes"},
		{Name: "/gc/scan/globals:bytes"},
		{Name: "/gc/scan/stack:bytes"},
	}
}

// newGCFloorTuner installs the floor. While it is in force a soft memory limit
// of twice the floor bounds the heap even if the follow-up is late.
func newGCFloorTuner(floor uint64) *gcFloorTuner {
	tuner := &gcFloorTuner{floor: floor, sample: newGCFloorSamples()}
	percent, ok := tuner.percent()
	if !ok {
		return nil
	}
	if percent == 100 {
		// The live heap is already past half the floor: nothing to hold back.
		return nil
	}
	// Only the runtime default is ours to replace: an explicit policy such as
	// --gc-aggressive has already set the percent.
	if previous := debug.SetGCPercent(percent); previous != 100 {
		debug.SetGCPercent(previous)
		return nil
	}
	tuner.last = percent
	if backstop := int64(floor * 2); debug.SetMemoryLimit(-1) > backstop {
		tuner.priorLimit = debug.SetMemoryLimit(backstop)
		tuner.ownedLimit = backstop
	}
	return tuner
}

// run follows the live heap until it passes half the floor or maxAge elapses,
// then restores the ordinary policy and returns. It polls so the hand-over does
// not depend on the scheduling of a finalizer. The goroutine ends at the
// hand-over, at maxAge, or with the process.
func (t *gcFloorTuner) run(every, maxAge time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	deadline := time.NewTimer(maxAge)
	defer deadline.Stop()
	for {
		select {
		case <-ticker.C:
			if t.step() {
				return
			}
		case <-deadline.C:
			t.release()
			return
		}
	}
}

// percent reads the last measured live heap and root sizes and returns the
// collector percent for the floor.
func (t *gcFloorTuner) percent() (int, bool) {
	metrics.Read(t.sample)
	for _, sample := range t.sample {
		if sample.Value.Kind() != metrics.KindUint64 {
			return 0, false
		}
	}
	live := t.sample[0].Value.Uint64()
	roots := t.sample[1].Value.Uint64() + t.sample[2].Value.Uint64()
	return gcFloorPercent(t.floor, live, roots), true
}

// gcFloorPercent returns the collector percent that puts the next goal at the
// floor while the live heap is under half of it, and 100 above that. The
// runtime's goal is live + (live + roots) * percent / 100, and it sizes the
// first goal from a 4 MiB minimum heap.
func gcFloorPercent(floor, live, roots uint64) int {
	live = max(live, 4<<20)
	if floor/2 > live {
		return int((floor-live)*100/(live+roots)) + 1
	}
	return 100
}

// step applies one update. It reports true when the tuner is finished: the
// live heap passed half the floor, metrics are unavailable, or another policy
// (for example --gc-aggressive) changed the percent.
func (t *gcFloorTuner) step() bool {
	percent, ok := t.percent()
	if !ok || percent == 100 {
		t.release()
		return true
	}
	if percent == t.last {
		return false
	}
	previous := debug.SetGCPercent(percent)
	if previous != t.last {
		// Another policy owns the percent. Hand its value back and stand down.
		debug.SetGCPercent(previous)
		t.dropLimit()
		return true
	}
	t.last = percent
	return false
}

// release hands the collector back to the ordinary 100 percent growth unless
// another policy has taken the percent, and drops the memory-limit backstop.
func (t *gcFloorTuner) release() {
	if previous := debug.SetGCPercent(100); previous != t.last {
		debug.SetGCPercent(previous)
	}
	t.dropLimit()
}

// dropLimit removes the backstop if it is still ours.
func (t *gcFloorTuner) dropLimit() {
	if t.ownedLimit != 0 && debug.SetMemoryLimit(-1) == t.ownedLimit {
		debug.SetMemoryLimit(t.priorLimit)
	}
	t.ownedLimit = 0
}
