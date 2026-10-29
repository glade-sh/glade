package apextest

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestObservedPhaseDurationSamples(t *testing.T) {
	phases := []struct {
		name  string
		field func(*runPerfCounters) *atomic.Int64
		want  func(int64) RunnerPhasePerfCounters
	}{
		{
			name:  "teardown",
			field: func(c *runPerfCounters) *atomic.Int64 { return &c.phases.teardownNS },
			want:  func(n int64) RunnerPhasePerfCounters { return RunnerPhasePerfCounters{TeardownNS: n} },
		},
		{
			name:  "history",
			field: func(c *runPerfCounters) *atomic.Int64 { return &c.phases.historyApplyNS },
			want:  func(n int64) RunnerPhasePerfCounters { return RunnerPhasePerfCounters{HistoryApplyNS: n} },
		},
		{
			name:  "report",
			field: func(c *runPerfCounters) *atomic.Int64 { return &c.phases.reportAssemblyNS },
			want:  func(n int64) RunnerPhasePerfCounters { return RunnerPhasePerfCounters{ReportAssemblyNS: n} },
		},
	}
	samples := []struct {
		name   string
		values []time.Duration
		wantNS int64
	}{
		{name: "zero", values: []time.Duration{0}, wantNS: 1},
		{name: "one-nanosecond", values: []time.Duration{time.Nanosecond}, wantNS: 1},
		{name: "positive", values: []time.Duration{7 * time.Millisecond}, wantNS: 7_000_000},
		{name: "negative", values: []time.Duration{-time.Nanosecond}, wantNS: -1},
		{name: "accumulate", values: []time.Duration{2 * time.Nanosecond, 0, 5 * time.Nanosecond}, wantNS: 8},
		{name: "two-zero-samples", values: []time.Duration{0, 0}, wantNS: 2},
		{name: "retain-negative-sample", values: []time.Duration{5 * time.Nanosecond, -2 * time.Nanosecond}, wantNS: 3},
	}
	for _, phase := range phases {
		t.Run(phase.name, func(t *testing.T) {
			for _, sample := range samples {
				t.Run(sample.name, func(t *testing.T) {
					counters := newRunPerfCounters(true)
					for _, value := range sample.values {
						counters.recordObservedPhaseDuration(phase.field(counters), value)
					}
					if got := snapshotPerfCounters(counters).Phases; got != phase.want(sample.wantNS) {
						t.Fatalf("recorded phases = %#v, want %#v", got, phase.want(sample.wantNS))
					}
				})
			}
			t.Run("inactive", func(t *testing.T) {
				counters := newRunPerfCounters(true)
				if got := snapshotPerfCounters(counters).Phases; got != (RunnerPhasePerfCounters{}) {
					t.Fatalf("unobserved phases = %#v, want zero", got)
				}
			})
			t.Run("disabled", func(t *testing.T) {
				counters := newRunPerfCounters(false)
				field := phase.field(counters)
				counters.recordObservedPhaseDuration(field, 0)
				counters.recordObservedPhaseDuration(field, time.Millisecond)
				if got := field.Load(); got != 0 {
					t.Fatalf("disabled phase storage = %d, want zero", got)
				}
				if got := snapshotPerfCounters(counters).Phases; got != (RunnerPhasePerfCounters{}) {
					t.Fatalf("disabled phases = %#v, want zero", got)
				}
			})
		})
	}
	t.Run("unstarted-method-window", func(t *testing.T) {
		counters := newRunPerfCounters(true)
		recordMethodWindow(time.Time{}, counters)
		if got := snapshotPerfCounters(counters).Phases; got != (RunnerPhasePerfCounters{}) {
			t.Fatalf("unstarted method window = %#v, want zero", got)
		}
	})
	t.Run("nil-counter", func(t *testing.T) {
		var counters *runPerfCounters
		var phase atomic.Int64
		counters.recordObservedPhaseDuration(&phase, 0)
		if got := phase.Load(); got != 0 {
			t.Fatalf("nil counter phase storage = %d, want zero", got)
		}
	})
}
