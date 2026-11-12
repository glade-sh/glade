package apextest

import "testing"

// An Apex action that throws inside a record-triggered flow with no fault
// connector fails the save with APEX_FAILED and the flow's process-failure
// text. The callout check ("uncommitted work pending") reaches the flow
// through the same path.
func TestFlowApexActionFaultNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "flowApexThrow", "flowApexThrowSave", "flowApexThrowCaught")
}

// A callout from a flow's Apex action after the flow's own DML fails the save
// with the uncommitted work error.
func TestFlowApexActionCalloutNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "flowCallout", "flowCalloutAfterStart")
}
