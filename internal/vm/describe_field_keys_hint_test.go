package vm

import "testing"

// The pre-size hint must cover a typical field describe so the map is built
// without growth, and not exceed it by much.
func TestDescribeFieldResultKeyHintCoversResult(t *testing.T) {
	machine := New(nil)
	org := describeOverlayOrg(4)
	machine.SetOrg(&org)
	for _, field := range []string{"pkg__Field_0001__c", "pkg__Status__c", "pkg__Stage__c"} {
		desc, err := machine.describeFieldValue("pkg__Widget__c", field)
		if err != nil {
			t.Fatal(err)
		}
		n := len(desc.Fields)
		if n > describeFieldResultKeyHint || n < describeFieldResultKeyHint*3/4 {
			t.Fatalf("%s has %d keys; hint %d should be within 25%% above", field, n, describeFieldResultKeyHint)
		}
	}
}
