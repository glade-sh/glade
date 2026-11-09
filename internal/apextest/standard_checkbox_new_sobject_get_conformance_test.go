package apextest

import "testing"

// a standard Boolean read through SObject.get on an unpopulated
// record is null natively (new, Schema newSObject, inserted caller, clone),
// while dotted reads stay false.
func TestStandardCheckboxNewSObjectGetAnonymousIsTestNamedVMNativeConformance(t *testing.T) {
	runRuntimeShapeFixture(t, "testdata/conformance/standard_checkbox_new_sobject_get", "StdCheckboxProbe", "StdCheckboxProbeTest", 14, 0, nil)
}
