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

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Every expectation is a black-box observation from the owned synthetic project.
// The named replay runs twice to cover reused project/runtime caches as well.
func TestSOQLCountOrderNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/soql_count_order"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma, reader.LazyQuotes = '\t', true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 149 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	compileSource, err := os.ReadFile(filepath.Join(fixture, "compile-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var compileCases map[string]string
	if err := json.Unmarshal(compileSource, &compileCases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
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
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project compiled: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			compare := func(t *testing.T, message, want string) {
				t.Helper()
				_, payload, ok := strings.Cut(message, "OBS|")
				if !ok {
					t.Fatalf("missing observation: %s", message)
				}
				var gotValue, wantValue any
				if json.Unmarshal([]byte(payload), &gotValue) != nil || json.Unmarshal([]byte(want), &wantValue) != nil {
					t.Fatalf("invalid JSON: %s / %s", payload, want)
				}
				if !reflect.DeepEqual(gotValue, wantValue) {
					t.Errorf("native=%s; Glade=%s", want, payload)
				}
			}
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatalf("invalid row: %v", row)
				}
				if row[0] != api || row[3] == "compile" || row[1] != "anonymous" {
					continue
				}
				row := row
				t.Run("anonymous/"+row[2]+"/"+row[3], func(t *testing.T) {
					source := "System.assert(false,'OBS|'+JSON.serialize(QueryShapeProbe.observe('" + row[2] + "','" + row[3] + "')));"
					if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					_, err = runner.execute(program)
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
						t.Fatalf("observation: %v", err)
					}
					compare(t, runtimeErr.ExceptionMessage(), row[4])
				})
			}
			t.Setenv("GLADE_HOME", t.TempDir())
			for _, phase := range []string{"cold", "warm"} {
				t.Run(phase, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"QueryShapeProbeTest"}, Parallelism: 1})
					problems := map[string]string{}
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								problems[c.MethodName] = c.Problem.Message
							}
						}
					}
					if len(problems) != 30 {
						t.Fatalf("named observations=%d: %+v", len(problems), run.Summary())
					}
					for _, row := range rows[1:] {
						if row[0] != api || row[1] != "isTest" || row[3] == "compile" {
							continue
						}
						t.Run(row[2]+"/"+row[3], func(t *testing.T) { compare(t, problems[row[2]+"_"+row[3]], row[4]) })
					}
				})
			}
			for _, row := range rows[1:] {
				if row[0] != api || row[3] != "compile" {
					continue
				}
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					source, ok := compileCases[row[2]]
					if !ok {
						t.Fatal("missing source")
					}
					var want struct {
						Compiled bool   `json:"compiled"`
						Message  string `json:"message"`
					}
					if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
						t.Fatal(err)
					}
					checkDiagnostic := func(diagnostics []diagnostic.Diagnostic) {
						t.Helper()
						if want.Compiled {
							return
						}
						_, missing, ok := strings.Cut(want.Message, "Variable does not exist: ")
						if !ok || missing == "" {
							t.Fatal("missing native field diagnostic")
						}
						for _, d := range diagnostics {
							if d.NativeMessage == "Variable does not exist: "+missing || d.Code == "GLADESEMA021" && strings.Contains(d.Message, missing) {
								return
							}
						}
						t.Errorf("native rejects missing field %s; Glade diagnostics=%v", missing, diagnostics)
					}
					if row[1] == "anonymous" {
						for _, phase := range []string{"cold", "warm"} {
							analysis := sema.AnalyzeAnonymous(index, source, api)
							checkDiagnostic(analysis.Diagnostics)
							if got := !analysis.HasErrors(); got != want.Compiled {
								t.Errorf("%s: native compiled=%v; Glade=%v diagnostics=%v", phase, want.Compiled, got, analysis.Diagnostics)
							}
						}
					} else {
						path := filepath.Join(root, "force-app/main/default/classes/ShapeCompileProbeTest")
						writeFile(t, path+".cls", "@IsTest public class ShapeCompileProbeTest { @IsTest static void checkField() { "+source+" } }")
						writeFile(t, path+".cls-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
						namedIndex := loadTestIndex(t, root)
						analysis := sema.Analyze(namedIndex)
						checkDiagnostic(analysis.Diagnostics)
						if got := !namedIndex.HasErrors() && !analysis.HasErrors(); got != want.Compiled {
							t.Errorf("native compiled=%v; Glade=%v diagnostics=%v %v", want.Compiled, got, namedIndex.Diagnostics, analysis.Diagnostics)
						}
					}
				})
			}
		})
	}
}
