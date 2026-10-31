package sema

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Complete schema rows are Salesforce observations; partial schemas are an
// offline boundary, because the source project need not contain every field.
func TestStandardFieldAccessNativeRoutes(t *testing.T) {
	fixture := "../apextest/testdata/conformance/standard_field_access"
	source, err := os.ReadFile(filepath.Join(fixture, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(source, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 25 {
		t.Fatalf("expected 24 observations, got %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			body := cases[row[2]]
			if body == "" {
				t.Fatal("missing case")
			}
			root := t.TempDir()
			file := filepath.Join(root, "FieldProbeTest.cls")
			writeSemaFile(t, file, "@IsTest public class FieldProbeTest { @IsTest static void runCase() { "+body+" } }")
			fields := []schema.Field{{Name: "ProbeField__c", Type: "Text"}}
			for _, partial := range []bool{false, true} {
				shape := schema.Schema{Objects: []schema.Object{{Name: "Account", Partial: partial, CompleteSnapshot: true, Fields: fields}, {Name: "Contact", Partial: partial, CompleteSnapshot: true, Fields: fields}}}
				snapshot, err := json.Marshal(shape)
				if err != nil {
					t.Fatal(err)
				}
				snapshot = []byte(strings.ReplaceAll(string(snapshot), `"completeSnapshot"`, `"fieldsComplete"`))
				path := filepath.Join(root, "snapshot.json")
				if err := os.WriteFile(path, snapshot, 0600); err != nil {
					t.Fatal(err)
				}
				digest := sha256.Sum256(snapshot)
				writeSemaFile(t, filepath.Join(root, "glade.yml"), fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %x\n", digest))
				p := project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: []string{file}, SchemaSnapshot: path, SchemaSnapshotSHA256: fmt.Sprintf("%x", digest)}
				loaded, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, loaded)
				for _, phase := range []string{"cold", "warm"} {
					result := Analyze(index)
					if row[1] == "anonymous" {
						result = AnalyzeAnonymous(index, body, row[0])
					}
					want := row[3] == "true" || partial
					if got := !result.HasErrors(); got != want {
						t.Errorf("partial=%t %s: native compiled=%t got=%t diagnostics=%v", partial, phase, want, got, result.Diagnostics)
					}
					if !want && !strings.Contains(row[4], "Variable does not exist:") {
						t.Fatalf("unexpected native rejection %q", row[4])
					}
				}
			}
		})
	}
}

// A standard-object metadata overlay does not prove the rest of the org's
// custom fields are absent. This is a local incomplete-input boundary.
func TestStandardFieldAccessOpenMetadata(t *testing.T) {
	for _, object := range []string{"Account", "Contact"} {
		t.Run(object, func(t *testing.T) {
			shape := schema.Schema{Objects: []schema.Object{{Name: object, Fields: []schema.Field{{Name: "Overlay__c", Type: "Text"}}}}}
			index := typesys.Build(project.Project{}, shape)
			result := AnalyzeAnonymous(index, "Object x = new "+object+"().InstalledField__c;", "67.0")
			if result.HasErrors() {
				t.Fatalf("overlay is not a complete org snapshot: %v", result.Diagnostics)
			}
		})
	}
}
