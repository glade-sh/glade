package vm

import "testing"

// Native rows dml_before_after, dml_inside_after, pre_active_after and
// inside_active_after establish the independent DML/savepoint boundary contract.
func TestCalloutSavepointBoundaryStopUnitRoute(t *testing.T) {
	for _, route := range []string{"before_dml", "inside_dml", "before_savepoint", "inside_savepoint"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			mark := func() {
				if route == "before_dml" || route == "inside_dml" {
					machine.calloutPendingWork.pending = true
				} else {
					machine.saveCalloutPendingWork("unit-point")
				}
			}
			if route == "before_dml" || route == "before_savepoint" {
				mark()
			}
			if _, err := machine.testStart(); err != nil {
				t.Fatal(err)
			}
			if route == "inside_dml" || route == "inside_savepoint" {
				mark()
			}
			if _, err := machine.testStop(&Result{}); err != nil {
				t.Fatal(err)
			}
			if err := machine.checkCalloutAllowed(); err != nil {
				t.Fatalf("native permits callouts after stopTest: %v", err)
			}
		})
	}
}
