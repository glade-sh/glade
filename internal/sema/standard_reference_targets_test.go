package sema

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// A project that customizes Assessment or DocumentGenerationProcess and
// declares any org feature gets the catalog fields merged into its schema.
// Their reference targets must be admitted (native Assessment describes list
// AssessmentStagedData; DocumentGenerationProcess lists DocGenerationBatchProcess).
func TestCatalogFieldsOfCustomizedIndustriesObjectsKeepReferenceTargets(t *testing.T) {
	for _, features := range [][]string{nil, {"EnableSetPasswordInApi"}} {
		name := "no-features"
		if len(features) > 0 {
			name = "features"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Probe.cls")
			writeSemaFile(t, path, "public class Probe { public static void run() { Assessment a = new Assessment(); DocumentGenerationProcess p = new DocumentGenerationProcess(); } }")
			objects := []schema.Object{
				{Name: "Assessment", Fields: []schema.Field{{Name: "Note__c", Type: "Text"}}},
				{Name: "DocumentGenerationProcess", Fields: []schema.Field{{Name: "Note__c", Type: "Text"}}},
			}
			index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{Objects: objects})
			index.OrgShapeFeatures = features
			result := Analyze(index)
			for _, diag := range result.Diagnostics {
				if diag.Code == "GLADESEMA003" {
					t.Fatalf("unknown reference target: %s", diag.Message)
				}
			}
		})
	}
}

// Every reference target in the standard describe catalog that is not a known
// standard object is admitted as a reference-only name. A custom field that
// references any of them must not raise GLADESEMA003.
func TestReferenceOnlyStandardTargetsAreAdmittedBySema(t *testing.T) {
	names := storage.ReferenceOnlyStandardObjectNames()
	if len(names) != 99 {
		t.Fatalf("reference-only names = %d, want 99", len(names))
	}
	sample := map[string]bool{}
	for _, name := range []string{"AssessmentStagedData", "DocGenerationBatchProcess", "ActivationTarget", "DataAction", "DataStream", "ExpressionSetDefinition", "BusinessUnit"} {
		sample[name] = false
	}
	var fields []schema.Field
	for i, name := range names {
		if _, ok := sample[name]; ok {
			sample[name] = true
		}
		fields = append(fields, schema.Field{Name: fmt.Sprintf("Ref%d__c", i), Type: "Lookup", ReferenceTo: []string{name}})
	}
	for name, seen := range sample {
		if !seen {
			t.Fatalf("sample target %s is not in the derived set", name)
		}
	}
	root := t.TempDir()
	path := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, path, "public class Probe { public static void run() { } }")
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{{Name: "Holder__c", Fields: fields}}})
	for _, diag := range Analyze(index).Diagnostics {
		if diag.Code == "GLADESEMA003" {
			t.Fatalf("unknown reference target: %s", diag.Message)
		}
	}
	// A name outside the derived set is still rejected.
	bad := typesys.Build(project.Project{Root: root, ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{{Name: "Holder__c", Fields: []schema.Field{{Name: "Ref__c", Type: "Lookup", ReferenceTo: []string{"NotAnObjectAnywhere"}}}}}})
	rejected := false
	for _, diag := range Analyze(bad).Diagnostics {
		rejected = rejected || diag.Code == "GLADESEMA003"
	}
	if !rejected {
		t.Fatal("unknown target NotAnObjectAnywhere was admitted")
	}
}
