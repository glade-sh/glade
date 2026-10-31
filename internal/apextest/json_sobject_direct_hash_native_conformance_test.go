package apextest

import "testing"

func TestJSONSObjectDirectHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "json_sobject_direct_hash", "JsonHash62Test", "62.0", "67.0", "68.0")
}
