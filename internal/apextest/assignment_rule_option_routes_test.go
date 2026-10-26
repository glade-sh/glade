package apextest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAssignmentRuleCaseLeadInsertUpdateOptionRoutes(t *testing.T) {
	const fixture = "testdata/conformance/dml_policies"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	expressions := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")[1:] {
		key, expression, ok := strings.Cut(line, "\t")
		if ok && strings.HasPrefix(key, "assignment_") {
			expressions[key] = expression
		}
	}
	if len(expressions) != 24 {
		t.Fatal("incomplete assignment case table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(fixture, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(strings.ReplaceAll(string(raw), "67.0", api), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) > 0 {
				t.Fatal(triggerErrors)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			var own []TestCase
			for _, row := range Discover(index, Options{SelectedClasses: []string{"DmlPolicyObservationTest"}}) {
				if expressions[row.MethodName] != "" {
					own = append(own, row)
				}
			}
			if len(own) != 24 {
				t.Fatalf("discovered %d/24 assignment methods", len(own))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, own)
			programs, programErrors := compileTestInvokePrograms(own)
			native := map[string]map[string]string{}
			for _, route := range []string{"anonymous", "istest"} {
				dir := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
				facts := read(dir + "RUN-FACTS.txt")
				for _, fact := range []string{"api=" + api, "context=" + route, "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+facts, "\n"+fact+"\n") {
						t.Fatalf("missing native fact %s", fact)
					}
				}
				native[route] = map[string]string{}
				for _, line := range strings.Split(strings.TrimSpace(read(dir+"org.tsv")), "\n")[1:] {
					key, value, ok := strings.Cut(line, "\t")
					if ok && expressions[key] != "" {
						native[route][key] = value
					}
				}
				if len(native[route]) != 24 {
					t.Fatal("incomplete native route")
				}
			}
			assertObservation := func(t *testing.T, key, message string, want string) {
				t.Helper()
				_, value, ok := strings.Cut(message, "OBSERVED|"+key+"|")
				if !ok || value != want {
					t.Fatalf("native %s; Glade %s", want, message)
				}
			}
			for _, row := range own {
				t.Run(row.MethodName, func(t *testing.T) {
					for _, route := range []string{"anonymous", "named_istest"} {
						t.Run(route, func(t *testing.T) {
							machine := runner.newMachine()
							machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
							key := testCaseKey(row)
							var execution error
							if route == "anonymous" {
								program, err := vm.CompileAnonymousWithOptions("System.assert(false,'OBSERVED|"+row.MethodName+"|'+"+expressions[row.MethodName]+");", vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, execution = machine.Execute(program)
							} else {
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("compile %v %v", methodErrors[key], programErrors[key])
								}
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, execution = machine.ExecuteTestInClass(programs[key], row.ClassName)
							}
							var fault *vm.RuntimeError
							if !errors.As(execution, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
								t.Fatalf("observation failed: %v", execution)
							}
							nativeRoute := "anonymous"
							if route == "named_istest" {
								nativeRoute = "istest"
							}
							assertObservation(t, row.MethodName, fault.ExceptionMessage(), native[nativeRoute][row.MethodName])
						})
					}
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"DmlPolicyObservationTest"}, NoDiskCache: true, Parallelism: 1}, own)
				count := 0
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						count++
						t.Run(row.MethodName, func(t *testing.T) {
							if row.Problem == nil || !strings.HasSuffix(row.Problem.Type, "AssertException") {
								t.Fatalf("missing observation %+v", row.Problem)
							}
							assertObservation(t, row.MethodName, row.Problem.Message, native["istest"][row.MethodName])
						})
					}
				}
				if count != 24 {
					t.Fatalf("runner observed %d/24 routes", count)
				}
			})
		})
	}
}
