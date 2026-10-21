package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestFixtureBoundaryNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/legacy_fixture_boundaries_native"
	file, err := os.Open(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 7 || strings.Join(rows[0], ",") != "api,case,route,result" {
		t.Fatalf("incomplete native boundary observations: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		if (row[0] != "62.0" && row[0] != "67.0") || seen[key] {
			t.Fatal("invalid or duplicate native row")
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			if row[2] == "deploy" {
				body, err := os.ReadFile(filepath.Join(directory, "rejected-controls", "BranchReturnControl.cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "force-app", "BranchReturnControl.cls"), string(body))
				analysis := sema.Analyze(loadTestIndex(t, root))
				if !analysis.HasErrors() || !strings.Contains(strings.ToLower(analysis.Diagnostics[0].Message), "unreachable") {
					t.Fatalf("native rejects unreachable return: %s; Glade: %+v", row[3], analysis.Diagnostics)
				}
				return
			}
			if row[2] != "istest" {
				t.Fatal("unknown native route")
			}
			project := filepath.Join(directory, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(project, path)
				if err == nil {
					writeFile(t, filepath.Join(root, relative), strings.ReplaceAll(string(body), "67.0", row[0]))
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"GroveSettingObservation"}, SelectedMethod: row[1], NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			_, payload, found := strings.Cut(problem.Message, "OBSERVED|"+row[1]+"|")
			var got, want map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
				t.Fatalf("invalid observation: %+v", problem)
			}
			if expected, ok := want["exceptionMessage"].(string); ok {
				actual, _ := got["exceptionMessage"].(string)
				for _, signature := range []string{"MISSING_ARGUMENT", "Id not specified in an update call"} {
					if !strings.Contains(expected, signature) || !strings.Contains(actual, signature) {
						t.Fatalf("native error=%s; Glade error=%s", expected, actual)
					}
				}
				delete(got, "exceptionMessage")
				delete(want, "exceptionMessage")
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", row[3], payload)
			}
		})
	}
}
