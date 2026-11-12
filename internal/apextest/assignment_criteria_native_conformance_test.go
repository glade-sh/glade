package apextest

import "testing"

// Assignment rule criteria operators and value rules on Lead. The criteria
// row holds one result per rule entry: 1 when the record went to the rule's
// queue, 0 when it kept its owner.
func TestAssignmentRuleCriteriaNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "criteria")
}
