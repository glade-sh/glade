package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSameSpelledCustomFieldNativeConformance(t *testing.T) {
	const fixtures = "testdata/same_spelled_custom_field"
	raw, err := os.ReadFile(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	if strings.Join(rows[0], "\t") != "api\tcase\taccepted\tobservation" {
		t.Fatalf("unexpected native header: %v", rows[0])
	}
	observations := map[string]string{"read": "seed", "assign": "sprout", "lowercase": "control"}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
		accepted, err := strconv.ParseBool(row[2])
		wantObservation, known := observations[row[1]]
		if err != nil || (accepted && (!known || row[3] != wantObservation)) || (!accepted && (row[1] != "missing" || !strings.Contains(row[3], "Variable does not exist: Absent__c"))) {
			t.Fatalf("invalid native outcome: %v", row)
		}
		key := row[0] + "/" + row[1]
		if seen[key] {
			t.Fatal("duplicate native row: " + key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			className := "GardenFieldRead" + strings.TrimSuffix(row[0], ".0")
			source, err := os.ReadFile(filepath.Join(fixtures, "project", "force-app", "main", "default", "classes", className+".cls"))
			if err != nil {
				t.Fatal(err)
			}
			if !accepted {
				className = "GardenMissingField" + strings.TrimSuffix(row[0], ".0")
				source = []byte("public class " + className + " { public static Object read() { Contact contact = new Contact(LastName='Row'); return Contact.Absent__c; } }")
			}
			for _, view := range []struct {
				name, namespace, field string
			}{
				{"unqualified", "", "Marker__c"},
				{"project-local", "garden", "Marker__c"},
				{"project-qualified", "garden", "garden__Marker__c"},
			} {
				t.Run(view.name, func(t *testing.T) {
					result := analyzeSameSpelledCustomField(t, className, string(source), row[0], view.namespace, view.field)
					if got := !result.HasErrors(); got != accepted {
						t.Fatalf("native accepted=%t observation=%q; Glade accepted=%t diagnostics=%+v", accepted, row[3], got, result.Diagnostics)
					}
					if !accepted && !sameSpelledCustomFieldHasMissingDiagnostic(result, "Absent__c") {
						t.Fatalf("missing-field native diagnostic was lost: %+v", result.Diagnostics)
					}
				})
			}
			if accepted {
				t.Run("foreign-field-control", func(t *testing.T) {
					result := analyzeSameSpelledCustomField(t, className, string(source), row[0], "garden", "foreign__Marker__c")
					if !sameSpelledCustomFieldHasMissingDiagnostic(result, "Marker__c") {
						t.Fatalf("another namespace's field satisfied an unqualified local field read: %+v", result.Diagnostics)
					}
				})
			}
		})
	}
}

func analyzeSameSpelledCustomField(t *testing.T, className, source, api, namespace, fieldName string) Result {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, className+".cls")
	writeSemaFile(t, path, source)
	index, artifacts := typesys.BuildWithArtifacts(project.Project{
		Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: []string{path},
	}, schema.Schema{Objects: []schema.Object{{
		Name: "Contact", Partial: true, Fields: []schema.Field{{Name: fieldName, Type: "Text", Length: 80}},
	}}})
	return AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
}

func sameSpelledCustomFieldHasMissingDiagnostic(result Result, field string) bool {
	for _, item := range result.Diagnostics {
		if item.Severity == diagnostic.Error && (item.Message == "Variable does not exist: "+field || item.NativeMessage == "Variable does not exist: "+field) {
			return true
		}
	}
	return false
}
