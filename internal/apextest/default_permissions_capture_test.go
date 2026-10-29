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

// The retained sources were deployed without profile or permission-set metadata.
// Each payload is native evidence, including default modifiers, lowercase twins,
// two independent custom objects, empty stripping, and explicit system controls.
func TestDefaultPermissionsCapturedRoutes(t *testing.T) {
	const family = "testdata/conformance/default_permissions_capture"
	raw, err := os.Open(filepath.Join(family, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	reader := csv.NewReader(raw)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 85 || strings.Join(rows[0], ",") != "source_api,method,observation" {
		t.Fatal("incomplete native matrix")
	}
	anonymous, err := os.ReadFile(filepath.Join(family, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(family, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, dir := range []string{"profiles", "permissionsets"} {
				if _, err := os.Stat(filepath.Join(root, "force-app/main/default", dir)); !os.IsNotExist(err) {
					t.Fatalf("unexpected grant metadata %s: %v", dir, err)
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; local diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: []string{"CedarDefaultAccessTest", "CedarExplicitAccessTest"}})
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			byName := map[string]TestCase{}
			for _, c := range cases {
				byName[c.MethodName] = c
			}
			if len(byName) != 12 {
				t.Fatalf("discovered %d methods", len(byName))
			}
			seen := map[string]bool{}
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				name := row[1]
				if seen[name] {
					t.Fatalf("duplicate native row %s", name)
				}
				seen[name] = true
				t.Run(name, func(t *testing.T) {
					compare := func(t *testing.T, message string) {
						t.Helper()
						_, payload, ok := strings.Cut(message, "OBSERVED|"+name+"|")
						var got, want any
						if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[2]), &want) != nil {
							t.Fatalf("invalid transport: %s", message)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("native=%s; Glade=%s", row[2], payload)
						}
					}
					if name == "anonymousRunAs" {
						body, err := os.ReadFile(filepath.Join(family, "anonymousRunAs.apex"))
						if err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) {
							t.Fatalf("missing native rejection: %v", err)
						}
						payload, err := json.Marshal(map[string]any{"rejected": true, "compiled": true, "compileProblem": "", "exceptionMessage": userModeInaccessibleExceptionType(runtimeErr.Type) + ": " + runtimeErr.ExceptionMessage()})
						if err != nil {
							t.Fatal(err)
						}
						compare(t, "OBSERVED|"+name+"|"+string(payload))
						return
					}
					if name == "anonymous" {
						program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := runner.newMachine()
						for _, pass := range []string{"cold", "warm"} {
							t.Run(pass, func(t *testing.T) {
								result, err := machine.Execute(program)
								if err != nil {
									t.Fatal(err)
								}
								found := false
								for _, line := range result.Debug {
									if strings.Contains(line, "OBSERVED|anonymous|") {
										compare(t, line)
										found = true
									}
								}
								if !found {
									t.Fatal("missing anonymous observation")
								}
							})
						}
						return
					}
					c, ok := byName[name]
					if !ok {
						t.Fatal("missing named route")
					}
					key := testCaseKey(c)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("compile %v %v", methodErrors[key], programErrors[key])
					}
					t.Run("isTest", func(t *testing.T) {
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, err := machine.ExecuteInClass(programs[key], c.ClassName)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing native assertion transport: %v", err)
						}
						compare(t, runtimeErr.ExceptionMessage())
					})
					// Repeated Run calls exercise the shared in-memory runtime cache.
					for _, pass := range []string{"project", "projectWarm"} {
						t.Run(pass, func(t *testing.T) {
							run := Run(index, Options{SelectedClasses: []string{c.ClassName}, SelectedMethod: name, NoDiskCache: true, Parallelism: 1})
							if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
								t.Fatalf("missing runner observation %+v", run.Summary())
							}
							result := run.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing runner transport %+v", result.Problem)
							}
							compare(t, result.Problem.Message)
						})
					}
				})
			}
			if len(seen) != 14 {
				t.Fatal("missing source API observations")
			}
		})
	}
}
