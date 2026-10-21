package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestDatabaseResultTokensNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "database_result_tokens")
	for _, route := range []string{"named", "anonymous"} {
		file := "org.tsv"
		if route == "anonymous" {
			file = "anonymous.tsv"
		}
		f, err := os.Open(filepath.Join(dir, file))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(f)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 3 || strings.Join(rows[0], ",") != "source_api,method,observation" {
			t.Fatal("incomplete native matrix")
		}
		for i, row := range rows[1:] {
			if row[0] != []string{"62.0", "67.0"}[i] || row[1] != "observe" {
				t.Fatal("unexpected native row", row)
			}
			t.Run(route+"/"+row[0], func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join(dir, "project")
				err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					source, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(project, path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(source), "62.0", row[0]))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				var message string
				if route == "named" {
					run := Run(index, Options{SelectedClasses: []string{"ResultTokenProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected one observation: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil {
						t.Fatal("missing native assertion transport")
					}
					if !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("%s: %s", result.Problem.Type, result.Problem.Message)
					}
					message = result.Problem.Message
				} else {
					source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = row[0]
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					result, err := runner.newMachine().Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					for _, line := range result.Debug {
						if strings.Contains(line, "OBSERVED|observe|") {
							message = line
						}
					}
				}
				_, payload, found := strings.Cut(message, "OBSERVED|observe|")
				var got, want map[string]any
				if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
					t.Fatalf("invalid observation: %s", message)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("native=%s; Glade=%s", row[2], payload)
				}
			})
		}
	}
}
