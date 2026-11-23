package apextest

import "testing"

// NATIVE automatic Create Records output assigned to lookup: org.tsv lines 2-5,
// anonymous and isolated tests at API 62.0 and 67.0.
func TestFlowCreatedRecordLookupNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_created_record_lookup", 4, nil)
}

// NATIVE before-save record update honors its filter: org.tsv lines 2-5,
// anonymous and isolated tests at API 62.0 and 67.0.
func TestFlowFilterUpdateRecordNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_filter_update_record", 4, nil)
}
