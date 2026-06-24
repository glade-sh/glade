package vm

import (
	"testing"
	"time"
)

func TestStaticAliasTopFieldsRankByDurationAndTrackOutcomes(t *testing.T) {
	ResetPerfCounters()
	SetPerfCountersEnabled(true)
	t.Cleanup(ResetPerfCounters)

	recordStaticAliasFieldPerf("Registry.fast", "map", 4, time.Millisecond, true)
	recordStaticAliasFieldPerf("Registry.slow", "object", 2, 3*time.Millisecond, false)
	recordStaticAliasFieldPerf("Registry.fast", "map", 7, 2*time.Millisecond, false)

	fields := SnapshotPerfCounters().StaticAliasTopFields
	if len(fields) != 2 {
		t.Fatalf("field count = %d, want 2: %#v", len(fields), fields)
	}
	if fields[0].Field != "Registry.fast" {
		t.Fatalf("first field = %q, want Registry.fast", fields[0].Field)
	}
	if fields[0].DurationNS != int64(3*time.Millisecond) {
		t.Fatalf("fast duration = %d, want %d", fields[0].DurationNS, int64(3*time.Millisecond))
	}
	if fields[0].Visits != 2 || fields[0].Changed != 1 || fields[0].NoChange != 1 {
		t.Fatalf("fast counters = %#v, want 2 visits, 1 changed, 1 no-change", fields[0])
	}
	if fields[0].MaxChildren != 7 {
		t.Fatalf("fast max children = %d, want 7", fields[0].MaxChildren)
	}
	if fields[1].Field != "Registry.slow" || fields[1].DurationNS != int64(3*time.Millisecond) {
		t.Fatalf("second field = %#v, want Registry.slow with 3ms", fields[1])
	}
}
