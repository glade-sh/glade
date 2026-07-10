package sema

import (
	"runtime"
	"time"
)

// PhaseCounters records aggregate work for one semantic-analysis phase.
type PhaseCounters struct {
	Calls      uint64
	DurationNS uint64
}

// PerfCounters records opt-in measurements for one Analyze call. Callers own
// the value and must set Enabled before passing it in AnalyzeOptions.
//
// Mallocs, TotalAllocBytes, GCCount, and GCPauseNS are deltas from
// runtime.ReadMemStats. They observe the entire Go process during the Analyze
// interval, including unrelated concurrent work. Use a quiescent or exclusive
// measurement environment when attributing those values to an Analyze call.
//
// Concurrent Analyze calls must use distinct PerfCounters output pointers.
// Sharing one output pointer concurrently is unsupported. Read an output only
// after its Analyze call returns.
type PerfCounters struct {
	Enabled                bool
	TotalNS                uint64
	SourceSchemaEnrichment PhaseCounters
	PlatformModel          PhaseCounters
	TypeMemberModel        PhaseCounters
	MethodBodies           PhaseCounters
	Inheritance            PhaseCounters
	QuerySemantics         PhaseCounters
	Export                 PhaseCounters
	Mallocs                uint64
	TotalAllocBytes        uint64
	GCCount                uint64
	GCPauseNS              uint64
}

type perfRecorder struct {
	output   *PerfCounters
	enabled  bool
	started  time.Time
	startMem perfMemSnapshot
	counters PerfCounters
}

type perfMemSnapshot struct {
	mallocs    uint64
	totalAlloc uint64
	numGC      uint32
	pauseNS    uint64
}

func newPerfRecorder(output *PerfCounters) perfRecorder {
	if output == nil || !output.Enabled {
		return perfRecorder{}
	}
	recorder := perfRecorder{
		output:  output,
		enabled: true,
		started: time.Now(),
		counters: PerfCounters{
			Enabled: true,
		},
	}
	recorder.startMem = readPerfMemSnapshot()
	return recorder
}

//go:noinline
func readPerfMemSnapshot() perfMemSnapshot {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return perfMemSnapshot{
		mallocs:    stats.Mallocs,
		totalAlloc: stats.TotalAlloc,
		numGC:      stats.NumGC,
		pauseNS:    stats.PauseTotalNs,
	}
}

func (r *perfRecorder) beginPhase() time.Time {
	if r == nil || !r.enabled {
		return time.Time{}
	}
	return time.Now()
}

func (r *perfRecorder) endPhase(phase *PhaseCounters, started time.Time) {
	if r == nil || !r.enabled {
		return
	}
	phase.Calls++
	phase.DurationNS += uint64(time.Since(started))
}

func (r *perfRecorder) finish() {
	if !r.enabled {
		return
	}
	endMem := readPerfMemSnapshot()
	r.counters.TotalNS = uint64(time.Since(r.started))
	r.counters.Mallocs = endMem.mallocs - r.startMem.mallocs
	r.counters.TotalAllocBytes = endMem.totalAlloc - r.startMem.totalAlloc
	r.counters.GCCount = uint64(endMem.numGC - r.startMem.numGC)
	r.counters.GCPauseNS = endMem.pauseNS - r.startMem.pauseNS
	*r.output = r.counters
}
