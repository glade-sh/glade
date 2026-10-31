package dml_test

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Preserve the shipping behavior for uncaptured multi-record custom-key
// upserts. This is a local compatibility control, not a Salesforce claim.
func TestUpsertDuplicateBulkCustomKeyPreservesShippingBehavior(t *testing.T) {
	loaded, err := project.Load(filepath.Join("../apextest/testdata/conformance/dml_policies", "project"))
	if err != nil {
		t.Fatal(err)
	}
	policies, err := automation.LoadProject(loaded)
	if err != nil || len(policies.Diagnostics) != 0 {
		t.Fatalf("policies: %v %v", err, policies.Diagnostics)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, key := range []string{"RouteKey__c", "OtherKey__c"} {
			t.Run(api+"/"+key, func(t *testing.T) {
				org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Contact")
				contact := org.Objects["Contact"]
				contact.Definition.Fields[key] = storage.Field{APIName: key, Type: storage.FieldString, ExternalID: true}
				org.Objects["Contact"] = contact
				automation.ApplyToOrg(&org, policies)
				engine := dml.NewEngine(&org)
				engine.Options.EnforceDuplicateRules = true
				row := func(value string) storage.Record {
					return storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Block"), "Email": storage.StringValue("duplicate@example.invalid"), key: storage.StringValue(value)}}
				}
				seed := engine.Insert([]storage.Record{row("seed")})
				if len(seed) != 1 || !seed[0].Success {
					t.Fatalf("seed: %+v", seed)
				}
				got := engine.UpsertWithExternalID([]storage.Record{row("first"), row("second")}, key)
				if len(got) != 2 || !got[0].Success || !got[1].Success || len(org.Objects["Contact"].Records) != 3 {
					t.Fatalf("shipping multi-record behavior: results=%+v count=%d", got, len(org.Objects["Contact"].Records))
				}
			})
		}
	}
}
