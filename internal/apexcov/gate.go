package apexcov

import "fmt"

// GatePercent is the deployment coverage floor.
const GatePercent = 75

// OverallGate applies the deployment rule for RunLocalTests and
// RunAllTestsInOrg: the line-weighted share over every class and trigger,
// floored, must reach 75. 74.5 percent fails and reports 74. Individual
// classes may sit far below 75. It returns the floored percent, whether the
// gate passes, and Salesforce's warning when it does not.
func OverallGate(parts []Counts) (percent int, ok bool, warning string) {
	percent = OrgWidePercent(parts)
	if percent >= GatePercent {
		return percent, true, ""
	}
	return percent, false, fmt.Sprintf("Average test coverage across all Apex Classes and Triggers is %d%%, at least %d%% test coverage is required.", percent, GatePercent)
}

// TriggerGate is the per-trigger rule under RunLocalTests: a trigger needs at
// least one covered line. A trigger at 0.33 percent passes; a trigger with no
// covered line fails even when the overall gate passes.
func TriggerGate(c Counts) (ok bool, warning string) {
	if c.Covered >= 1 {
		return true, ""
	}
	return false, "Test coverage of selected Apex Trigger is 0%, at least 1% test coverage is required"
}

// SpecifiedClassGate is the per-class rule under RunSpecifiedTests: each
// selected class must reach 75 percent on its own, whatever the org total.
func SpecifiedClassGate(c Counts) (ok bool, warning string) {
	percent := 0
	if c.Total > 0 {
		percent = 100 * c.Covered / c.Total
	}
	if percent >= GatePercent {
		return true, ""
	}
	return false, fmt.Sprintf("Test coverage of selected Apex Class is %d%%, at least %d%% test coverage is required", percent, GatePercent)
}
