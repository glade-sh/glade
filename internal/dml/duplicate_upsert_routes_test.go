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

func TestDuplicateUpsertEngineInsertRoutesNativeRows(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	for _, api := range []string{"62", "67"} {
		rows, err := os.ReadFile(filepath.Join(fixture, "routes/native/api"+api+"/anonymous/org.tsv"))
		if err != nil || !strings.Contains(string(rows), "upsert_keyword\tDUPLICATES_DETECTED|1\n") {
			t.Fatalf("missing native upsert row: %v", err)
		}
		for _, route := range []string{"implicit_id", "explicit_id"} {
			t.Run(api+"/"+route, func(t *testing.T) {
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
				engine := dml.NewEngine(&org)
				engine.Options.EnforceDuplicateRules = true
				row := storage.Record{Object: "Contact", Fields: map[string]storage.Value{
					"LastName": storage.StringValue("Block"), "Email": storage.StringValue("upsert@example.invalid"),
				}}
				if result := engine.Insert([]storage.Record{row}); len(result) != 1 || !result[0].Success {
					t.Fatalf("seed: %+v", result)
				}
				var results []dml.Result
				if route == "explicit_id" {
					results = engine.UpsertWithExternalID([]storage.Record{row}, "Id")
				} else {
					results = engine.Upsert([]storage.Record{row})
				}
				if len(results) != 1 || results[0].Success || results[0].StatusCode != "DUPLICATES_DETECTED" {
					t.Fatalf("native duplicate rejection; Glade=%+v", results)
				}
				if got := len(org.Objects["Contact"].Records); got != 1 {
					t.Fatalf("native retains one Contact; got %d", got)
				}
			})
		}
	}
}
