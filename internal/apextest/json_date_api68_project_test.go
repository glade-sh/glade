package apextest

import (
	"encoding/json"
	"testing"
)

// Replay the captured 62/67/68 named and test-method Date controls through
// the project runner, including the ordinary and strict null variants.
func TestJSONDateAPI68CapturedProject(t *testing.T) {
	index := loadTestIndex(t, "testdata/conformance/api68plumb_date_probe/named-project")
	run := Run(index, Options{})
	if got := run.Summary(); got.Total != 12 || got.Passed != 12 {
		data, _ := json.Marshal(run)
		t.Fatalf("captured Date project: %s", data)
	}
}
