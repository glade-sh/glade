package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRollupCheckboxFilterNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/legacy_fixture_boundaries_native"
	file, err := os.Open(filepath.Join(dir, "rollup-org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || strings.Join(rows[0], ",") != "source_api,method,observation" {
		t.Fatal("expected observations at source APIs 62.0 and 67.0")
	}
	for i, api := range []string{"62.0", "67.0"} {
		row := rows[i+1]
		if row[0] != api || row[1] != "checkboxNotEqualCount" {
			t.Fatal("missing checkbox filter observation")
		}
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "rollup-project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "67.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"GroveRollupObservation"}, SelectedMethod: row[1], NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing assertion transport: %+v; problem: %+v", result, result.Problem)
			}
			_, payload, found := strings.Cut(result.Problem.Message, "OBSERVED|"+row[1]+"|")
			var got, want map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
				t.Fatalf("invalid observation: %s", result.Problem.Message)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", row[2], payload)
			}
		})
	}
}
