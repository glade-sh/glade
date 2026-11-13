package apextest

import "testing"

// Assignment rule criteria operators and value rules on Lead. The criteria
// row holds one result per rule entry: 1 when the record went to the rule's
// queue, 0 when it kept its owner.
func TestAssignmentRuleCriteriaNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "criteria")
}

// A lead that an assignment rule entry with startsWith criteria routes to a
// queue, saved with the duplicate rule header and change data capture enabled:
// the entry matches, the owner changes and one create event is delivered.
func TestAssignmentRuleStartsWithNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "cdcAssign", "dupAssignAlertAllowSave")
}
