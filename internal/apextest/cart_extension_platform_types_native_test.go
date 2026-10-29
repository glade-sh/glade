package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// The captured named declarations pass through the project test runner's
// compile gate even when their platform-dependent methods are not called.
func TestCartExtensionPlatformTypesProjectNamedNativeConformance(t *testing.T) {
	const dir = "../sema/testdata/cart_extension_platform_types"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]struct{ ID, Name, Source string }, len(cases))
	for _, c := range cases {
		sources[c.ID] = c
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(string(raw)))
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, row := range rows[1:] {
		if row[1] != "deploy" {
			continue
		}
		c, ok := sources[row[2]]
		if !ok || (row[3] != "accepted" && row[3] != "rejected") {
			t.Fatalf("invalid named capture: %v", row)
		}
		checked++
		t.Run(row[0]+"/project-isTest/"+row[2], func(t *testing.T) {
			files := map[string]string{
				c.Name + ".cls":                 c.Source,
				"GardenPlatformCompileTest.cls": "@IsTest private class GardenPlatformCompileTest { @IsTest static void observe() { " + c.Name + " probe = new " + c.Name + "(); System.assertNotEquals(null, probe); } }",
			}
			index := nativeBoundaryIndex(t, row[0], files, nil)
			run := Run(index, Options{SelectedClasses: []string{"GardenPlatformCompileTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			summary := run.Summary()
			if row[3] == "rejected" {
				if summary.CompileErrors == 0 {
					t.Fatalf("native rejected with %q; project compile gate accepted: %+v", row[4], summary)
				}
				return
			}
			if summary.Total != 1 || summary.Passed != 1 || summary.CompileErrors != 0 || len(run.Suites) != 1 || run.Suites[0].Cases[0].Status != testreport.StatusPass {
				t.Fatalf("native accepted; project test result=%+v; problem=%s", summary, firstRunProblem(run))
			}
		})
	}
	if checked != 16 {
		t.Fatalf("project native compile rows=%d, want 16", checked)
	}
}
