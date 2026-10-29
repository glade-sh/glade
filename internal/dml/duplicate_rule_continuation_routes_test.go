package dml_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDuplicateRulesContinueAfterPermittedMatchEngineNativeRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	for _, api := range []string{"62", "67"} {
		for _, operation := range []string{"insert", "update"} {
			t.Run(api+"/"+operation, func(t *testing.T) {
				rows, err := os.ReadFile(filepath.Join(fixture, "native/api"+api+"/anonymous/org.tsv"))
				if err != nil || !strings.Contains(string(rows), "duplicate_Block_"+operation+"_options_true\tDUPLICATES_DETECTED|false\n") {
					t.Fatalf("missing native blocking-rule observation: %v", err)
				}
				loaded, err := project.Load(filepath.Join(fixture, "project"))
				if err != nil {
					t.Fatal(err)
				}
				policies, err := automation.LoadProject(loaded)
				if err != nil || len(policies.Diagnostics) != 0 {
					t.Fatalf("load policies: %v %v", err, policies.Diagnostics)
				}
				org := storage.OrgState{APIVersion: api + ".0", Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Contact")
				automation.ApplyToOrg(&org, policies)
				storage.EnsureDeterministicPlatformData(&org)
				if got := len(dml.ActiveDuplicateRules(&org, "Contact")); got < 3 {
					t.Fatalf("default and both custom rules must be installed; got %d", got)
				}
				engine := dml.NewEngine(&org)
				engine.Options.EnforceDuplicateRules = true
				engine.Options.AllowDuplicateSave = true
				seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{
					"LastName": storage.StringValue("Block"), "Email": storage.StringValue("later-rule@example.invalid"),
				}}
				if saved := engine.Insert([]storage.Record{seed}); len(saved) != 1 || !saved[0].Success {
					t.Fatalf("seed: %+v", saved)
				}
				candidate := seed.Clone()
				if operation == "update" {
					candidate.Fields["Email"] = storage.StringValue("different@example.invalid")
					saved := engine.Insert([]storage.Record{candidate})
					if len(saved) != 1 || !saved[0].Success {
						t.Fatalf("update seed: %+v", saved)
					}
					candidate.ID = saved[0].ID
					candidate.Fields["Email"] = seed.Fields["Email"]
				}
				var results []dml.Result
				if operation == "update" {
					results = engine.Update([]storage.Record{candidate})
				} else {
					results = engine.Insert([]storage.Record{candidate})
				}
				if len(results) != 1 || results[0].Success || results[0].StatusCode != "DUPLICATES_DETECTED" {
					t.Fatalf("permitted default match bypassed the custom blocking rule: %+v", results)
				}
			})
		}
	}
}
