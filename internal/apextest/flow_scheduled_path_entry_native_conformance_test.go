package apextest

import "testing"

// A scheduled path does not run inside a test, whatever its time source: not
// for a time already past, not at zero offset from the trigger, and not for an
// update that newly meets the entry condition.
func TestFlowScheduledPathEntryNativeConformance(t *testing.T) {
	runAutomationResidualRows(t, "schedPast", "schedExec", "schedUpdate")
}
