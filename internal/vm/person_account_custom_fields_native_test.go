package vm

import (
	"io"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// The native fixture inserts its Business Account sponsor before observing
// person_insert. It declares Contact.OptIn__c with a false checkbox default.
func TestPersonAccountCustomFieldsNativeDefaultRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"insert", "upsert_insert"} {
			for _, person := range []bool{false, true} {
				name := "business"
				if person {
					name = "person"
				}
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureDeterministicPlatformData(&org)
					contact := org.Objects["Contact"]
					contact.Definition = contact.Definition.Clone()
					contact.Definition.Fields["OptIn__c"] = storage.Field{APIName: "OptIn__c", Type: storage.FieldBoolean, DefaultValue: "false"}
					org.Objects["Contact"] = contact
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
					records := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Garden Sponsor")}}}
					if person {
						records[0].Fields = map[string]storage.Value{"LastName": storage.StringValue("Gardener")}
					}
					machine := New(io.Discard)
					machine.Org = &org
					machine.applySObjectFieldDefaults(records, "")
					engine := dml.NewEngine(&org)
					var results []dml.Result
					if route == "insert" {
						results = engine.Insert(records)
					} else {
						results = engine.Upsert(records)
					}
					if len(results) != 1 || !results[0].Success {
						t.Fatalf("native fixture inserts the sponsor and person: %+v", results)
					}
				})
			}
		}
	}
}
