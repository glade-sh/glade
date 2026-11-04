package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Explicit types are captured at case349_profile_boundary/org.tsv:27-29.
func TestCase349ExternalPreflightExplicitTypeNative(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", Active: true, Available: true}, {ID: "012000000000002", Active: true, Available: true, PersonAccount: true}}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		rt     storage.ID
		status string
	}{{"012000000000002", "INVALID_FIELD_FOR_INSERT_UPDATE"}, {"012000000000001", "INVALID_FIELD_FOR_INSERT_UPDATE"}, {"012000000000099", "INVALID_CROSS_REFERENCE_KEY"}} {
		records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("new"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic"), "RecordTypeId": storage.IDValue(tc.rt)}}}
		if got := machine.preflightBusinessAccountNameFailures(records, "upsert", "ProbeKey__c"); len(got) != 1 || got[0].Success || got[0].StatusCode != tc.status {
			t.Errorf("explicit type %s: %#v, want %s", tc.rt, got, tc.status)
		} else if tc.status == "INVALID_CROSS_REFERENCE_KEY" && got[0].Error != "Record Type ID: this ID value isn't valid for the user: "+string(tc.rt) {
			t.Errorf("invalid type diagnostic = %q", got[0].Error)
		}
	}
}
