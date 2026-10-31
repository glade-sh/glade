package apextest

import (
	"encoding/csv"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestReceiverDependencyControlsNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/receiver_dependency_controls"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 || strings.Join(rows[0], ",") != "source_api,route,case,observation" {
		t.Fatalf("invalid native matrix: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			projectDir := filepath.Join(dir, "project")
			err := filepath.WalkDir(projectDir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(projectDir, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("loaded project diagnostics: %v %v", index.Diagnostics, a.Diagnostics)
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				key := row[1] + "/" + row[2]
				if seen[key] {
					t.Fatalf("duplicate row %s", key)
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					switch row[1] {
					case "istest":
						result := Run(index, Options{SelectedClasses: []string{"ReceiverControlTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
						if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
							t.Fatalf("unexpected results: %+v", result.Summary())
						}
						problem := result.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing transport: %+v", problem)
						}
						_, payload, found := strings.Cut(problem.Message, "OBSERVED|"+row[2]+"|")
						var got, want any
						if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil || !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%s local=%s", row[3], payload)
						}
					case "anonymous":
						body, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
						if err != nil {
							t.Fatal(err)
						}
						if analysis := sema.AnalyzeAnonymous(index, string(body), api); analysis.HasErrors() {
							t.Fatalf("anonymous diagnostics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						org := orgFromIndex(index)
						org.APIVersion = api
						runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
						for i := 0; i < 2; i++ {
							if _, err := runner.execute(program); err != nil {
								t.Fatalf("anonymous execution %d: %v", i, err)
							}
						}
						if row[3] != "SUCCESS" {
							t.Fatalf("unexpected native outcome %s", row[3])
						}
					case "deploy":
						missingRoot := t.TempDir()
						writeFile(t, filepath.Join(missingRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"`+api+`"}`)
						body, err := os.ReadFile(filepath.Join(dir, "ReceiverMissingControl.cls"))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(missingRoot, "classes", "ReceiverMissingControl.cls"), string(body))
						writeFile(t, filepath.Join(missingRoot, "classes", "ReceiverMissingControl.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						missing := loadTestIndex(t, missingRoot)
						analysis := sema.Analyze(missing)
						if !missing.HasErrors() && !analysis.HasErrors() {
							t.Fatal("missing dependency should reject deployment")
						}
						if row[3] != "Invalid type: MissingReceiverDependency" {
							t.Fatalf("unexpected native rejection %s", row[3])
						}
					default:
						t.Fatalf("unknown route %s", row[1])
					}
				})
			}
			if len(seen) != 4 {
				t.Fatalf("incomplete routes: %v", seen)
			}
		})
	}
}
