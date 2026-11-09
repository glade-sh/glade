package apextest

import "testing"

// Cross-object and nested Text formulas on Trigger.new and Trigger.old records
// (org capture trigger_record_formulas, API 62.0 and 67.0, anonymous and isTest):
// a cross-object formula follows the record's own lookup value and the parent's
// current value; a nested formula reads the referenced formula untrimmed; BR()
// is the 12-character text _BR_ENCODED_ inside a formula, rendered <br> only
// from SOQL; and the formula values of a before-update record do not change
// after the trigger assigns an input. Every leaf is asserted.
func TestTriggerRecordFormulasNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutesParts(t, "trigger_record_formulas", "TfProbeTest", true, []string{"anonymous", "isTest"}, compareFormulaObservationLeaves, "62.0", "67.0")
}
