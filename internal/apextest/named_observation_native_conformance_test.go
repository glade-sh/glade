package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func runNamedObservationNativeConformance(t *testing.T, family, class string, apiVersions ...string) {
	t.Helper()
	fixture := filepath.Join("testdata", "conformance", family)
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(apiVersions)+1 || strings.Join(rows[0], ",") != "source_api,route,observation" {
		t.Fatal("incomplete native named-test matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 3 || !slices.Contains(apiVersions, row[0]) || row[1] != "isTest" || seen[row[0]] {
			t.Fatalf("invalid native cell: %v", row)
		}
		seen[row[0]] = true
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected one observation: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v; problem: %+v", result, result.Problem)
			}
			_, observed, _ := strings.Cut(result.Problem.Message, "OBS|")
			var got, want any
			if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid observation: %q", observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}
