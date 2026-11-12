package gladecli

import (
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
)

// testGCFloorBytes is the heap size below which `glade test` does not start a
// collection. A cold test process builds its org and class catalog from nothing
// and the live heap climbs from a few MiB to several hundred. With the default
// collector target every doubling is a full mark of everything built so far.
// Holding the next-collection goal at this floor skips those cycles. Once the
// live heap passes half the floor the target is the ordinary 100 percent
// growth, so the peak heap of a large run is unchanged.
const testGCFloorBytes = 512 << 20

// TuneTestGC applies the heap floor to the standalone `glade test` process.
// An explicit GOGC always wins. A soft memory limit under four times the floor
// lowers the floor to a quarter of the limit. Library callers and test
// binaries keep the runtime default.
func TuneTestGC(args []string) {
	if len(args) == 0 || args[0] != "test" || os.Getenv("GOGC") != "" {
		return
	}
	startGCFloor(testGCFloorBytes)
}

type gcFloorTuner struct {
	floor  uint64
	last   int
	sample []metrics.Sample
}

type gcFloorSentinel struct{ tuner *gcFloorTuner }

func newGCFloorSamples() []metrics.Sample {
	return []metrics.Sample{
		{Name: "/gc/heap/live:bytes"},
		{Name: "/gc/scan/globals:bytes"},
		{Name: "/gc/scan/stack:bytes"},
	}
}

func startGCFloor(floor uint64) {
	if limit := debug.SetMemoryLimit(-1); limit > 0 && uint64(limit)/4 < floor {
		floor = uint64(limit) / 4
	}
	if floor == 0 {
		return
	}
	tuner := &gcFloorTuner{floor: floor, sample: newGCFloorSamples()}
	percent, ok := tuner.percent()
	if !ok {
		return
	}
	tuner.last = percent
	debug.SetGCPercent(percent)
	armGCFloorSentinel(tuner)
}

// armGCFloorSentinel registers a finalizer on an object that dies at the next
// collection, so the tuner runs once per cycle.
func armGCFloorSentinel(tuner *gcFloorTuner) {
	sentinel := &gcFloorSentinel{tuner: tuner}
	runtime.SetFinalizer(sentinel, func(s *gcFloorSentinel) {
		if s.tuner.afterCycle() {
			armGCFloorSentinel(s.tuner)
		}
	})
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

// afterCycle runs once per completed collection. It reports whether the
// tuner stays armed.
func (t *gcFloorTuner) afterCycle() bool {
	percent, ok := t.percent()
	if !ok {
		return false
	}
	if percent == t.last {
		return true
	}
	previous := debug.SetGCPercent(percent)
	if previous != t.last {
		// Someone else set a policy (for example --gc-aggressive). Hand it back.
		debug.SetGCPercent(previous)
		return false
	}
	t.last = percent
	return true
}
