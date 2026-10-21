package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestUpsertClearedSelectorKeepsIDControl(t *testing.T) {
	org := testDataOrg()
	org.Objects["TriadParcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "TriadParcel__c", KeyPrefix: "a09", Fields: map[string]storage.Field{
			"Name":         {APIName: "Name", Type: storage.FieldString},
			"ParcelKey__c": {APIName: "ParcelKey__c", Type: storage.FieldString, ExternalID: true},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, cleared := range []bool{false, true} {
		row := Object("TriadParcel__c")
		row.Fields["Id"] = String("a09000000000001")
		markExplicitSObjectField(&row, "Id")
		if cleared {
			row.Fields["ParcelKey__c"] = Null
			markExplicitSObjectField(&row, "ParcelKey__c")
		}
		record, err := machine.recordFromValue(&row)
		if err != nil {
			t.Fatal(err)
		}
		if record.ID != "a09000000000001" || record.ExplicitNulls["ParcelKey__c"] != cleared {
			t.Fatalf("selector intent lost: %#v", record)
		}
	}
}
