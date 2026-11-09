package sema

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native aggregate and qualified SOQL: case289_followups/static-query.tsv.
func TestObjectPermissionsStaticQueryRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/case289_followups/"
	data, err := os.ReadFile(dir + "static-query.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dir + "static-query.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	// The fresh API 68 capture equals 67 for these cases (ORG-RUN.txt).
	// Replay the observed 67 results at 68 without duplicating oracle rows.
	for _, row := range append([][]string(nil), rows[1:]...) {
		if row[0] == "67.0" {
			copy := append([]string(nil), row...)
			copy[0] = "68.0"
			rows = append(rows, copy)
		}
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			index := typesys.Build(project.Project{SourceAPIVersion: row[0]}, schema.Schema{})
			result := AnalyzeAnonymous(index, cases[row[2]], row[0])
			want := row[3] == "false"
			if result.HasErrors() != want {
				t.Fatalf("native rejection=%t: %v", want, result.Diagnostics)
			}
			if want {
				if len(result.Diagnostics) != 1 {
					t.Fatalf("native reports one error: %v", result.Diagnostics)
				}
				message := result.Diagnostics[0].Message + result.Diagnostics[0].NativeMessage
				column := regexp.MustCompile(`Column:([0-9]+)`).FindString(row[5])
				if column == "" || !strings.Contains(message, column) {
					t.Fatalf("native column %q: %v", column, result.Diagnostics)
				}
				if strings.Contains(row[5], "Invalid field:") {
					if !strings.Contains(message, "Invalid field: 'PermissionsViewAllFields'") {
						t.Fatalf("native invalid-field message: %v", result.Diagnostics)
					}
				} else if !strings.Contains(message, "No such column 'PermissionsViewAllFields'") {
					t.Fatalf("native missing-column message: %v", result.Diagnostics)
				}
			}
		})
	}
}
