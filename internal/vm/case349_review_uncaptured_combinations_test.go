package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native profile controls cover the simple candidate and selected other
// signals (case349_profile_boundary/org.tsv:26-31). Unobserved combinations
// retain the ordinary DML path.
func TestCase349ExternalPreflightLeavesUncapturedCombinations(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	account := org.Objects["Account"]
	account.Definition.Fields["ProbeKey__c"] = storage.Field{APIName: "ProbeKey__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	machine := New(nil)
	machine.SetOrg(&org)
	for _, extra := range []string{"PersonDepartment", "PersonBirthdate", "Synthetic__c"} {
		t.Run(extra, func(t *testing.T) {
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("new"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic"), extra: storage.StringValue("Synthetic")}}
			if got := machine.preflightBusinessAccountNameFailures([]storage.Record{record}, "upsert", "ProbeKey__c"); len(got) > 0 && !got[0].Success {
				t.Fatalf("unobserved %s preflighted: %#v", extra, got)
			}
		})
	}
	for _, first := range []storage.Value{storage.NullValue(), storage.StringValue(""), storage.StringValue(" ")} {
		record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"ProbeKey__c": storage.StringValue("new"), "Name": storage.StringValue("Synthetic"), "FirstName": first}}
		if got := machine.preflightBusinessAccountNameFailures([]storage.Record{record}, "upsert", "ProbeKey__c"); len(got) > 0 && !got[0].Success {
			t.Errorf("unobserved FirstName %#v preflighted: %#v", first, got)
		}
	}
}
