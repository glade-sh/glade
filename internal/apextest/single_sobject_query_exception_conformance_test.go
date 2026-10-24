package apextest

import "testing"

func TestSingleSObjectQueryExceptionNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "single_sobject_query_exception", "SingleSObjectQueryExceptionTest", []string{"62.0", "64.0", "67.0"}, 7)
}
