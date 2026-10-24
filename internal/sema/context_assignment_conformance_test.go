package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Namespace and schema variants are local isolation checks. The native project
// is unnamespaced; these variants do not establish managed-package parity.
func TestContextAssignmentNativeConformance(t *testing.T) {
	const fixtures = "testdata/context_assignment"
	raw, err := os.ReadFile(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 37 {
		t.Fatalf("native matrix rows=%d err=%v", len(rows), err)
	}
	if strings.Join(rows[0], "\t") != "api\troute\tcase\taccepted\tobservation" {
		t.Fatal("invalid native header")
	}
	helper, err := os.ReadFile(filepath.Join(fixtures, "project/force-app/main/default/classes/AssignmentHelper.cls"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "named" && row[1] != "istest") || (row[3] != "true" && row[3] != "false") || row[4] == "" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] {
			t.Fatal("duplicate row: " + key)
		}
		seen[key] = true
		source, err := os.ReadFile(filepath.Join(fixtures, row[2]+".apex"))
		if err != nil {
			t.Fatal(err)
		}
		for _, variant := range []string{"baseline", "unmanaged-field", "managed-field", "same-spelled-unrelated-field", "project-namespace", "standard-date-metadata", "list-variable", "array-variable", "list-literal", "qualified-database", "managed-dependency", "helper-list-return", "helper-list-field", "helper-list-property"} {
			t.Run(key+"/"+variant, func(t *testing.T) {
				root := t.TempDir()
				p := project.Project{Root: root, SourceAPIVersion: row[0]}
				objects := []schema.Object{{Name: "Specimen__c", NameField: schema.NameField{Type: "Text"}}}
				body := string(source)
				switch variant {
				case "unmanaged-field":
					objects = append(objects, schema.Object{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Observed__c", Type: "DateTime"}}})
				case "managed-field":
					objects = append(objects, schema.Object{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "garden__Observed__c", Type: "Date"}}})
				case "same-spelled-unrelated-field":
					// A schema-index stress case, not deployable custom-field metadata.
					objects[0].Fields = []schema.Field{{Name: "Birthdate", Type: "DateTime"}}
				case "project-namespace":
					p.Namespace = "garden"
				case "standard-date-metadata":
					objects = append(objects, schema.Object{Name: "Contact", Partial: true, Fields: []schema.Field{{Name: "Birthdate", Type: "Date"}}})
				case "list-variable":
					if strings.Contains(body, "new List<Specimen__c>()") {
						body = "List<Specimen__c> records = new List<Specimen__c>(); " + strings.ReplaceAll(body, "new List<Specimen__c>()", "records")
					}
				case "array-variable":
					if strings.Contains(body, "new List<Specimen__c>()") {
						body = "Specimen__c[] records = new Specimen__c[]{}; " + strings.ReplaceAll(body, "new List<Specimen__c>()", "records")
					}
				case "list-literal":
					body = strings.ReplaceAll(body, "new List<Specimen__c>()", "new List<Specimen__c>{}")
				case "qualified-database":
					body = strings.ReplaceAll(body, "Database.", "System.Database.")
				}
				files := map[string]string{"AssignmentHelper.cls": string(helper)}
				switch variant {
				case "helper-list-return", "helper-list-field", "helper-list-property":
					member := "public static List<Specimen__c> records() { return new List<Specimen__c>(); }"
					replacement := "AssignmentRows.records()"
					if variant == "helper-list-field" {
						member = "public static List<Specimen__c> records = new List<Specimen__c>();"
						replacement = "AssignmentRows.records"
					}
					if variant == "helper-list-property" {
						member = "public static List<Specimen__c> records { get { return new List<Specimen__c>(); } }"
						replacement = "AssignmentRows.records"
					}
					files["AssignmentRows.cls"] = "public class AssignmentRows { " + member + " }"
					body = strings.ReplaceAll(body, "new List<Specimen__c>()", replacement)
				}
				if row[1] == "named" {
					files["AssignmentNamed.cls"] = "public class AssignmentNamed { public static void probe() { " + body + " } }"
				}
				if row[1] == "istest" {
					files["AssignmentAcceptedTest.cls"] = "@IsTest private class AssignmentAcceptedTest { @IsTest static void probe() { " + body + " } }"
				}
				for name, content := range files {
					path := filepath.Join(root, name)
					writeSemaFile(t, path, content)
					p.ApexFiles = append(p.ApexFiles, path)
				}
				if variant == "managed-dependency" {
					depRoot := filepath.Join(root, "dependency")
					if err := os.MkdirAll(depRoot, 0o755); err != nil {
						t.Fatal(err)
					}
					depPath := filepath.Join(depRoot, "NamespaceMarker.cls")
					writeSemaFile(t, depPath, "global class NamespaceMarker { global static Date today() { return Date.today(); } }")
					dep := project.Project{Root: depRoot, Namespace: "garden", SourceAPIVersion: row[0], ApexFiles: []string{depPath}}
					p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Namespace: "garden", SourceRoot: depRoot, Version: "1.0", Status: "loaded", Project: &dep}}
				}
				index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{Objects: objects})
				result := AnalyzeWithOptions(index, AnalyzeOptions{Diagnostics: true, BuildArtifacts: &artifacts})
				if row[1] == "anonymous" {
					result = AnalyzeAnonymous(index, body, row[0])
				}
				if variant == "managed-dependency" && row[1] != "anonymous" && row[3] == "false" {
					// Source-backed dependency analysis can downgrade semantic errors.
					// Check diagnostic retention without asserting native compile parity
					// for that local policy. The native rejection remains in org.tsv.
					if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA018") {
						t.Fatalf("native assignment rejection diagnostic lost: %+v", result.Diagnostics)
					}
					t.Logf("native rejects; source-dependency diagnostic retained; local hard error=%t", result.HasErrors())
					return
				}
				if got := !result.HasErrors(); got != (row[3] == "true") {
					t.Fatalf("native accepted=%s (%s); Glade accepted=%t diagnostics=%+v", row[3], row[4], got, result.Diagnostics)
				}
				if row[3] == "false" && !hasDiagnosticCode(result.Diagnostics, "GLADESEMA018") {
					t.Fatalf("assignment rejection lost: %+v", result.Diagnostics)
				}
			})
		}
	}
}
