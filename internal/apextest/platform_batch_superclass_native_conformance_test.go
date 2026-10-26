package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestPlatformBatchSuperclassProjectAnonymousAndIsTestNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/platform_batch_runtime"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Method, Anonymous string
		Routes            []string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	byMethod := map[string]string{}
	expectedKeys := map[string]bool{}
	for _, c := range cases {
		if c.Method == "" || len(c.Routes) == 0 {
			t.Fatal("incomplete source case")
		}
		byMethod[c.Method] = c.Anonymous
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range c.Routes {
				expectedKeys[api+"/"+route+"/"+c.Method] = true
			}
		}
	}
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(cases) != 5 || len(rows) != 19 || strings.Join(rows[0], ",") != "source_api,helper_api,route,method,observation" {
		t.Fatalf("incomplete native runtime matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[2] + "/" + row[3]
		if seen[key] || !expectedKeys[key] || (row[1] != "62.0" && row[1] != "67.0") {
			t.Fatalf("invalid native row %v", row)
		}
		seen[key] = true
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, helperAPI := range []string{"62.0", "67.0"} {
				applicable := false
				for _, row := range rows[1:] {
					if row[0] == api && row[1] == helperAPI {
						applicable = true
					}
				}
				if !applicable {
					continue
				}
				t.Run("helper-"+helperAPI, func(t *testing.T) {
					root := t.TempDir()
					project := filepath.Join(dir, "project")
					err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
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
						writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", helperAPI))
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					index := loadTestIndex(t, root)
					analysis := sema.Analyze(index)
					if index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("native project compiles; Glade=%v %v", index.Diagnostics, analysis.Diagnostics)
					}
					for _, row := range rows[1:] {
						if row[0] != api || row[1] != helperAPI {
							continue
						}
						t.Run(row[2]+"/"+row[3], func(t *testing.T) {
							var payload string
							marker := "OBSERVED|" + row[3] + "|"
							if row[2] == "istest" {
								run := Run(index, Options{SelectedClasses: []string{"PlatformBatchObservationTest"}, SelectedMethod: row[3], NoDiskCache: true, Parallelism: 1})
								if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
									t.Fatalf("missing test route: %+v", run.Summary())
								}
								problem := run.Suites[0].Cases[0].Problem
								if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
									t.Fatalf("missing native transport: %+v", problem)
								}
								_, payload, _ = strings.Cut(problem.Message, marker)
							} else {
								body := byMethod[row[3]]
								if body == "" {
									t.Fatal("missing anonymous source")
								}
								if result := sema.AnalyzeAnonymous(index, body, api); result.HasErrors() {
									t.Fatalf("native anonymous executes; Glade=%v", result.Diagnostics)
								}
								program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								org := orgFromIndex(index)
								org.APIVersion = api
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
								_, execErr := runner.execute(program)
								var runtimeErr *vm.RuntimeError
								if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
									t.Fatalf("missing native anonymous transport: %v", execErr)
								}
								_, payload, _ = strings.Cut(runtimeErr.ExceptionMessage(), marker)
							}
							var got, want any
							if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[4]), &want) != nil {
								t.Fatalf("invalid observation: native=%s Glade=%s", row[4], payload)
							}
							if !reflect.DeepEqual(got, want) {
								t.Fatalf("native=%s; Glade=%s", row[4], payload)
							}
						})
					}
				})
			}
		})
	}
}
