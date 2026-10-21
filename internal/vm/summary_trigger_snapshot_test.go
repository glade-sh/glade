package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Check the supplied trigger record snapshot rather than a fresh storage read.
// Native rollup-trigger old/new timing remains pending capture.
func TestTriggerSummaryReadPreservesSuppliedSnapshot(t *testing.T) {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["NumberIn__c"] = storage.Field{
		APIName: "NumberIn__c", Type: storage.FieldSummary, DisplayType: "DECIMAL",
		SummaryOperation: "count", SummaryForeignKey: "TriadChild__c.ParentLink__c",
	}
	org.Objects["Account"] = account
	org.Objects["TriadChild__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "TriadChild__c", KeyPrefix: "a01", Fields: map[string]storage.Field{
			"ParentLink__c": {APIName: "ParentLink__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		}},
		Records: map[storage.ID]storage.Record{"a01000000000001": {Object: "TriadChild__c", Fields: map[string]storage.Value{
			"ParentLink__c": storage.IDValue("001000000000001"),
		}}},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, count := range []int64{-1, 0, 1} {
		fields := map[string]storage.Value{}
		if count >= 0 {
			fields["NumberIn__c"] = storage.IntegerValue(count)
		}
		row := vmValueFromRecord(storage.Record{Object: "Account", ID: "001000000000001", Fields: fields})
		markTriggerSObject(&row)
		got, err := machine.lookupPath(row, []string{"NumberIn__c"})
		expected := count
		if expected < 0 {
			expected = 0
		}
		if err != nil || !got.Equal(Int(expected)) {
			t.Fatalf("snapshot %d: got %#v, error %v", count, got, err)
		}
	}
}
