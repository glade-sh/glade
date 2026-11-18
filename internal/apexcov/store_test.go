package apexcov

import (
	"fmt"
	"testing"
)

// NATIVE test_runner_coverage/RUN-FACTS.txt notes: a run of a test class
// replaces that class's per-test rows, so an earlier method's row vanishes,
// while aggregates only grow.
func TestStoreReplacesPerTestRowsAndGrowsAggregates(t *testing.T) {
	store := NewStore(map[string][]int{"CvUnion": {2, 3, 4, 5, 7, 9}})
	store.Record([]TestRun{{TestClass: "CvUnionTest", Method: "aLeft", Covered: map[string][]int{"CvUnion": {2, 3, 4, 5, 9}}}})
	store.Record([]TestRun{{TestClass: "CvUnionTest", Method: "bRight", Covered: map[string][]int{"CvUnion": {2, 3, 4, 7, 9}}}})
	if _, ok := store.PerTest("CvUnionTest", "aLeft", "CvUnion"); ok {
		t.Fatal("rerunning the test class kept an earlier method's row")
	}
	if got, ok := store.PerTest("cvuniontest", "BRIGHT", "CvUnion"); !ok || fmt.Sprint(got) != "[2 3 4 7 9]" {
		t.Fatalf("bRight row = %v %v", got, ok)
	}
	if got := store.Aggregate("CvUnion"); fmt.Sprint(got.Covered) != "[2 3 4 5 7 9]" || len(got.Uncovered) != 0 {
		t.Fatalf("aggregate = %+v", got)
	}
	store.Record([]TestRun{{TestClass: "OtherTest", Method: "none", Covered: map[string][]int{"CvUnion": {}}}})
	if got := store.Aggregate("CvUnion"); len(got.Covered) != 6 || store.OrgWide() != 100 {
		t.Fatalf("a later run shrank the aggregate: %+v", got)
	}
}
