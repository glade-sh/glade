package sema

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native rows: case289_followups/static-adjacent.tsv; API 68 equals 67.
func TestObjectPermissionsStaticAdjacentRoutes(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/case289_followups/static-adjacent.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(source, &cases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
		for name, body := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				for _, pass := range []string{"cold", "warm"} {
					result := AnalyzeAnonymous(index, body, api)
					if got, want := result.HasErrors(), api == "62.0"; got != want {
						t.Fatalf("%s: native rejection=%t, got %t: %v", pass, want, got, result.Diagnostics)
					}
					if api == "62.0" && len(result.Diagnostics) != 1 {
						t.Fatalf("%s: native reports one field rejection: %v", pass, result.Diagnostics)
					}
					if api == "62.0" && (name == "where" || name == "order") {
						column := "Column:40"
						if name == "order" {
							column = "Column:43"
						}
						if !strings.Contains(result.Diagnostics[0].Message, column) {
							t.Fatalf("native %s: %v", column, result.Diagnostics)
						}
					}
				}
			})
		}
	}
}
