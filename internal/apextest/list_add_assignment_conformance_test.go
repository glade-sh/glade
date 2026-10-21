package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestListAddAssignmentNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/list_add_assignment"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 7 || strings.Join(rows[0], ",") != "id,api,route,status,observed" {
		t.Fatalf("expected six native cells with their header, got %d rows", len(rows))
	}
	assignment := regexp.MustCompile(`^Illegal assignment from (\w+) to (\w+)$`)
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if row[0] != "ADD_ASSIGNMENT" || seen[key] || row[3] != "COMPILE_REJECTED" ||
			(row[1] != "62.0" && row[1] != "67.0" && row[1] != "68.0") ||
			(row[2] != "anonymous" && row[2] != "@IsTest") {
			t.Fatalf("invalid or duplicate native cell %s", key)
		}
		seen[key] = true
		t.Run(row[1]+"/"+row[2], func(t *testing.T) {
			var native struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(row[4]), &native); err != nil {
				t.Fatal(err)
			}
			types := assignment.FindStringSubmatch(native.Message)
			if len(types) != 3 {
				t.Fatalf("unexpected native assignment diagnostic %q", native.Message)
			}
			anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			result := sema.AnalyzeAnonymous(typesys.Index{}, string(anonymous), row[1])
			want := "initializes y with " + types[1]
			if row[2] == "@IsTest" {
				source, err := os.ReadFile(filepath.Join(fixture, "ListAddAssignmentTest.cls"))
				if err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				path := filepath.Join(root, "ListAddAssignmentTest.cls")
				writeFile(t, path, string(source))
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[1], ApexFiles: []string{path}}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				result = sema.Analyze(index)
				want = "initializes " + types[2] + " local \"y\" with " + types[1]
			}
			errors := 0
			for _, diag := range result.Diagnostics {
				if diag.Severity != "error" {
					continue
				}
				errors++
				if !strings.Contains(diag.Message, want) {
					t.Errorf("native %q; local %q, want %q", native.Message, diag.Message, want)
				}
			}
			if errors != 1 {
				t.Fatalf("expected one incompatible assignment diagnostic: %v", result.Diagnostics)
			}
		})
	}
}
