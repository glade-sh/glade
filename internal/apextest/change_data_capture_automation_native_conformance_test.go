package apextest

import "testing"

// Change events around assignment rules, flow-created records, duplicate rule
// outcomes, delete, undelete and merge. The merge rows differ between native
// runs (the surviving lead's update event appears in one with LastModifiedDate
// as its only changed field); every run reports the loser's delete event and
// no update event without a changed field.
func TestChangeDataCaptureAutomationNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "cdcDelUndel", "cdcFlowCreate", "cdcDupBlocked", "cdcDupAllowSave", "cdcMerge")
}
