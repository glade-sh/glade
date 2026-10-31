package apextest

import "testing"

// Replays the synthetic API 62/67 observations retained in org.tsv.
func TestInsertEventChildNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "insert_event_child", "RoadCapture62Test", "62.0", "67.0")
}
