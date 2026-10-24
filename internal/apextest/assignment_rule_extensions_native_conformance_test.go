package apextest

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

const assignmentExtensionFixture = platformTransactionFixture + "/capture_extensions"

type assignmentExtensionCase struct{ id, family string }

func readAssignmentExtensionCases(t *testing.T) []assignmentExtensionCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(assignmentExtensionFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 51 || lines[0] != "id\tfamily\tshape" {
		t.Fatal("incomplete path extension case table")
	}
	seen := map[string]bool{}
	var rows []assignmentExtensionCase
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 3 || cells[0] == "" || cells[2] == "" || seen[cells[0]] {
			t.Fatalf("invalid case %q", line)
		}
		seen[cells[0]] = true
		rows = append(rows, assignmentExtensionCase{cells[0], cells[1]})
	}
	return rows
}

func readAssignmentExtensionNative(t *testing.T, api, family, route string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(assignmentExtensionFixture, family, "native", "api"+strings.TrimSuffix(api, ".0"), route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "family=" + family + "\n", "context=" + route + "\n", "native_complete=true\n", "deployment_success=true\n", "fixture_permission_set_assigned=true\n", "matching_rules_active=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete capture: %s", required)
		}
	}
	for _, sourceRoot := range []string{"project"} {
		base := filepath.Join(assignmentExtensionFixture, sourceRoot)
		if err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(assignmentExtensionFixture, path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256([]byte(strings.ReplaceAll(string(raw), "62.0", api)))
			if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), digest)) {
				t.Errorf("native source changed: %s", rel)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"cases.tsv", "anonymous.apex"} {
		raw, err := os.ReadFile(filepath.Join(assignmentExtensionFixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256(raw))) {
			t.Fatalf("native source changed: %s", name)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(receipt), fmt.Sprintf("org_tsv_sha256=%x\n", sha256.Sum256(raw))) {
		t.Fatal("native rows changed")
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "id\tgroup\troute\tstatus\tvalue" {
		t.Fatal("invalid native header")
	}
	observations := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 || cells[1] != family || cells[2] != route || cells[3] != "runtime" || observations[cells[0]] != nil {
			t.Fatalf("invalid native row %s", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[4]), &value) != nil || value == nil {
			t.Fatal("invalid native JSON")
		}
		observations[cells[0]] = value
	}
	count := 0
	for _, row := range readAssignmentExtensionCases(t) {
		if row.family == family {
			count++
			if observations[row.id] == nil {
				t.Fatalf("missing native row %s", row.id)
			}
		}
	}
	if count != len(observations) || !strings.Contains(string(receipt), fmt.Sprintf("rows=%d\n", count)) {
		t.Fatal("unexpected native row count")
	}
	return observations
}

func TestAssignmentRuleExtensionNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, family := range []string{"assignment_rules"} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(family+"/"+api+"/"+route, func(t *testing.T) { readAssignmentExtensionNative(t, api, family, route) })
			}
		}
	}
}

func copyAssignmentExtensionProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(assignmentExtensionFixture, "project")
	if err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(raw), "62.0", api))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertAssignmentExtensionPayload(t *testing.T, id, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "PATH_OBS|"+id+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing native observation transport: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		native, _ := json.Marshal(want)
		t.Fatalf("native=%s; Glade=%s", native, payload)
	}
}

func assertAssignmentExtensionObservation(t *testing.T, id string, err error, want map[string]any) {
	t.Helper()
	var fault *vm.RuntimeError
	if !errors.As(err, &fault) {
		t.Fatalf("native observation completed; Glade=%v", err)
	}
	if want["uncaught"] == true {
		typeName := fault.Type
		if !strings.Contains(typeName, ".") {
			typeName = "System." + typeName
		}
		if typeName != want["exception"] || fault.ExceptionMessage() != want["message"] {
			t.Fatalf("native=%v; Glade=%v", want, err)
		}
		return
	}
	if !strings.HasSuffix(fault.Type, "AssertException") {
		t.Fatalf("native observation completed; Glade=%v", err)
	}
	assertAssignmentExtensionPayload(t, id, fault.ExceptionMessage(), want)
}

func TestAssignmentRuleExtensionNativeConformance(t *testing.T) {
	cases := readAssignmentExtensionCases(t)
	for _, family := range []string{"assignment_rules"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(family+"/"+api, func(t *testing.T) {
				anonymous := readAssignmentExtensionNative(t, api, family, "anonymous")
				named := readAssignmentExtensionNative(t, api, family, "istest")
				index := loadTestIndex(t, copyAssignmentExtensionProject(t, api))
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
				for _, row := range Discover(index, Options{SelectedClasses: []string{"PathObservationTest", "PathSetupObservationTest"}}) {
					if named[row.MethodName] != nil {
						own = append(own, row)
					}
				}
				if len(own) != len(named) {
					t.Fatalf("discovered %d native methods, want %d", len(own), len(named))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, own)
				programs, programErrors := compileTestInvokePrograms(own)
				template, err := os.ReadFile(filepath.Join(assignmentExtensionFixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				for _, row := range cases {
					if row.family != family {
						continue
					}
					t.Run(row.id+"/anonymous", func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(strings.ReplaceAll(string(template), "CASE_ID", row.id), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						// The native anonymous capture runs as the scratch administrator.
						machine := runner.newMachine()
						machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
						_, err = machine.Execute(program)
						assertAssignmentExtensionObservation(t, row.id, err, anonymous[row.id])
					})
				}
				for _, row := range own {
					t.Run(row.MethodName+"/named_istest", func(t *testing.T) {
						key := testCaseKey(row)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
						machine.EnableTestContext()
						if row.ClassName == "PathSetupObservationTest" {
							setup, err := vm.CompileAnonymousWithOptions("insert new Account(Name='oracle-setup');", vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err = machine.Execute(setup); err != nil {
								t.Fatal(err)
							}
						}
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err = machine.ExecuteTestInClass(programs[key], row.ClassName)
						assertAssignmentExtensionObservation(t, row.MethodName, err, named[row.MethodName])
					})
				}
				t.Run("project_test_runner", func(t *testing.T) {
					report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"PathObservationTest", "PathSetupObservationTest"}, NoDiskCache: true, Parallelism: 1}, own)
					results := map[string]testreport.Case{}
					for _, suite := range report.Suites {
						for _, row := range suite.Cases {
							results[row.MethodName] = row
						}
					}
					for _, row := range own {
						t.Run(row.MethodName, func(t *testing.T) {
							result, ok := results[row.MethodName]
							if !ok || result.Problem == nil {
								t.Fatalf("missing native observation; status=%s; problem=%+v; exclusions=%v", result.Status, result.Problem, report.ExcludedClasses)
							}
							if named[row.MethodName]["uncaught"] == true {
								if result.Problem.Type != named[row.MethodName]["exception"] || result.Problem.Message != named[row.MethodName]["message"] {
									t.Fatalf("native=%v; Glade=%+v", named[row.MethodName], result.Problem)
								}
								return
							}
							if !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("native observation completed; Glade=%+v", result.Problem)
							}
							assertAssignmentExtensionPayload(t, row.MethodName, result.Problem.Message, named[row.MethodName])
						})
					}
				})
			})
		}
	}
}
