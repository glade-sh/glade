package apextest

import "testing"

func TestSObjectListHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "sobject_list_hash", "ListHash62Test", "62.0", "67.0", "68.0")
}
