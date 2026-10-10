package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native H001-H005 at 62/67 accept source-owned Flow/lxscheduler inner types.
// TestAutomationOrgConformance retains the platform C016 rejection control.
func TestAutomationContractsRespectSourceOwnedShadows(t *testing.T) {
	var data struct {
		APIVersions  []string
		Declarations map[string]string
		Cases        []struct{ ID, Code, Expected string }
	}
	raw, err := os.ReadFile("testdata/automation_shadow_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.APIVersions) != 2 || data.APIVersions[0] != "62.0" || data.APIVersions[1] != "67.0" || len(data.Cases) != 5 {
		t.Fatalf("shadow oracle: %d APIs, %d rows", len(data.APIVersions), len(data.Cases))
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			paths := make([]string, 0, len(data.Declarations))
			for name, source := range data.Declarations {
				path := filepath.Join(root, name+".cls")
				writeSemaFile(t, path, source)
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				files := append([]string{}, paths...)
				if extra != "" {
					files = append(files, extra)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("shadow declarations: %v", index.Diagnostics)
			}
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("shadow declarations: %v", result.Diagnostics)
			}
			assertAccepted := func(t *testing.T, id, expected string, result Result) {
				t.Helper()
				observed := "ACCEPTED"
				if result.HasErrors() {
					observed = "REJECTED"
				}
				if expected != "ACCEPTED" || observed != expected {
					t.Fatalf("%s expected <%s> actual <%s>: %v", id, expected, observed, result.Diagnostics)
				}
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					body := "P pq=new P();\n" + tc.Code
					t.Run("anonymous", func(t *testing.T) {
						assertAccepted(t, tc.ID, tc.Expected, AnalyzeAnonymous(index, body, api))
					})
					t.Run("named", func(t *testing.T) {
						path := filepath.Join(root, "AutomationShadowProbe.cls")
						writeSemaFile(t, path, "public class AutomationShadowProbe { public static void run(){\n"+body+"\n} }")
						named := buildIndex(path)
						if named.HasErrors() {
							t.Fatalf("shadow row parser: %v", named.Diagnostics)
						}
						assertAccepted(t, tc.ID, tc.Expected, Analyze(named))
					})
				})
			}
		})
	}
}
