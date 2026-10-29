package apextest

import "testing"

func TestQueryConditionalRecordNativeConformance(t *testing.T) {
	runSyntheticObservationNativeConformance(t, "query_conditional_record", "QueryChoiceProbe", "observeChoice", "QUERY_CHOICE_RESULT", 7, false)
}
