package dml_test

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// A custom external-ID insertion retains the release engine's no-statement
// duplicate policy until that selector has its own native capture.
func TestUpsertCustomKeyDuplicateRetainsReleaseEngineBehavior(t *testing.T) {
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
			for _, collide := range []bool{false, true} {
				t.Run(api+"/"+key+"/"+map[bool]string{false: "distinct", true: "duplicate"}[collide], func(t *testing.T) {
					org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
					storage.EnsureStandardObject(&org, "Contact")
					contact := org.Objects["Contact"]
					contact.Definition.Fields[key] = storage.Field{APIName: key, Type: storage.FieldString, ExternalID: true}
					org.Objects["Contact"] = contact
					automation.ApplyToOrg(&org, policies)
					engine := dml.NewEngine(&org)
					engine.Options.EnforceDuplicateRules = true
					seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Block"), "Email": storage.StringValue("duplicate@example.invalid"), key: storage.StringValue("seed")}}
					if results := engine.Insert([]storage.Record{seed}); len(results) != 1 || !results[0].Success {
						t.Fatalf("seed: %+v", results)
					}
					row := seed.Clone()
					row.Fields[key] = storage.StringValue("new")
					if !collide {
						row.Fields["Email"] = storage.StringValue("distinct@example.invalid")
					}
					results := engine.UpsertWithExternalID([]storage.Record{row}, key)
					if len(results) != 1 || !results[0].Success || !results[0].Created {
						t.Fatalf("release custom-key insertion: %+v", results)
					}
					if len(org.Objects["Contact"].Records) != 2 {
						t.Fatal("custom-key insertion must retain both records")
					}
				})
			}
		}
	}
}
