package apextest

import "testing"

func TestSObjectHashVersionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "sobject_hash_version", "HashVersion62Test", "62.0", "67.0", "68.0")
}
