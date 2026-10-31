package apextest

import "testing"

func TestSObjectJSON3TypeofErrorsNativeConformance(t *testing.T) {
	runSObjectJSON2NativeConformance(t, "testdata/conformance/sobject_json3_typeof_errors", "TypeofErrorProbe", "SOBJECT_TYPEOF|", "", "ownerAccount ownerContact ownerOpportunity whoAccount validBoundary unknownType", "value")
}
