package apextest

import "testing"

// Duplicate rules for records that record-triggered flows create or update,
// and the DuplicateRuleHeader allowSave carried from the saving statement into
// the flow's Create Records and Update Records elements.
func TestFlowDuplicateRulesNativeConformance(t *testing.T) {
	runAutomationResidualRows(t,
		"dupBeforeFlow", "dupAfterFlow", "dupTrigger", "dupFlowAlert", "dupFlowAlertAllowSave",
		"dupFlowUpdate", "dupFlowCreateAllowSave", "dupApexUpdate",
		"dupAssignBlock", "dupAssignBlockAllowSave", "dupAssignAlert")
}
