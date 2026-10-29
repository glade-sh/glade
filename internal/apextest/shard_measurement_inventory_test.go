package apextest

import "testing"

// Completed expensive families must not silently receive the millisecond
// fallback merely because an earlier profiling prefix did not reach them.
func TestShardCompletedSlowFamilyWeights(t *testing.T) {
	durations, err := loadShardDurationTable(shardDurationTablePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TestHeapLimitsOrgConformance", "TestIterableAssignabilityOrgConformance", "TestNameResolutionOrgConformance", "TestEventListExtendedNativeConformance"} {
		if durations[name] < 1000 {
			t.Errorf("completed slow family %s missing measured weight: %d", name, durations[name])
		}
	}
}
