package apextest

import "testing"

func TestRecordTypeScopeNativeConformance(t *testing.T) {
	runSyntheticObservationNativeConformance(t, "record_type_scope", "RecordTypeScopeProbe", "observeScope", "RECORD_TYPE_SCOPE_RESULT", 4, true)
}
