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

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

const platformTransactionFixture = "testdata/conformance/platform_transaction_paths"

var platformTransactionFamilies = []string{"cdc", "flow_paths", "event_flows", "duplicate_rules", "assignment_rules", "callout_work"}

type platformTransactionCase struct{ id, family string }

func readPlatformTransactionCases(t *testing.T) []platformTransactionCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(platformTransactionFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 33 || lines[0] != "id\tfamily\tshape\troutes\tapis" {
		t.Fatal("incomplete platform transaction case table")
	}
	var cases []platformTransactionCase
	seen := map[string]bool{}
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 5 || cells[0] == "" || seen[cells[0]] || cells[2] == "" || cells[3] != "anonymous,istest" || cells[4] != "62.0,67.0" {
			t.Fatalf("invalid case: %s", line)
		}
		seen[cells[0]] = true
		cases = append(cases, platformTransactionCase{cells[0], cells[1]})
	}
	return cases
}

func readPlatformTransactionNative(t *testing.T, family, api, route string, cases []platformTransactionCase) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(platformTransactionFixture, family, "api"+strings.TrimSuffix(api, ".0"), "native", route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "family=" + family + "\n", "context=" + route + "\n", "native_complete=true\n", "deployment_success=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lines[0] != "id\tgroup\troute\tstatus\tvalue" {
		t.Fatal("invalid native header")
	}
	observations := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 || cells[1] != family || cells[2] != route || cells[3] != "runtime" || observations[cells[0]] != nil {
			t.Fatalf("invalid native row: %s", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[4]), &value) != nil || value == nil {
			t.Fatal("invalid native JSON")
		}
		observations[cells[0]] = value
	}
	count := 0
	for _, row := range cases {
		if row.family == family {
			count++
			if observations[row.id] == nil {
				t.Fatalf("missing native row %s", row.id)
			}
		}
	}
	countKey := "rows"
	if strings.Contains(string(receipt), "\nexported_rows=") {
		countKey = "exported_rows"
	}
	if len(observations) != count || !strings.Contains(string(receipt), fmt.Sprintf("%s=%d\n", countKey, count)) {
		t.Fatal("unexpected native rows")
	}
	verifyPlatformCaptureSetup(t, string(receipt))
	sourceRoot := filepath.Join(platformTransactionFixture, "project")
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		source := strings.ReplaceAll(string(raw), "62.0", api)
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256([]byte(source)))) {
			t.Errorf("native source changed: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	anonymous, err := os.ReadFile(filepath.Join(platformTransactionFixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(receipt), fmt.Sprintf("anonymous_template_sha256=%x\n", sha256.Sum256(anonymous))) {
		t.Fatal("anonymous source changed")
	}
	return observations
}

func TestPlatformTransactionPathNativeRows(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, family := range platformTransactionFamilies {
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(family+"/"+api+"/"+route, func(t *testing.T) { readPlatformTransactionNative(t, family, api, route, cases) })
			}
		}
	}
}

func copyPlatformTransactionProject(t *testing.T, family, api string) string {
	t.Helper()
	root := t.TempDir()
	sourceRoot := filepath.Join(platformTransactionFixture, "project")
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		// Unrelated metadata is disjoint: only its own family touches each object.
		if strings.Contains(slash, "/flows/") {
			if family == "event_flows" {
				if filepath.Base(path) != "CaptureEvent.flow-meta.xml" {
					return nil
				}
			} else if family == "flow_paths" {
				if filepath.Base(path) == "CaptureEvent.flow-meta.xml" {
					return nil
				}
			} else {
				return nil
			}
		}
		if strings.Contains(slash, "/triggers/") && family != "cdc" {
			return nil
		}
		if (strings.Contains(slash, "/matchingRules/") || strings.Contains(slash, "/duplicateRules/")) && family != "duplicate_rules" {
			return nil
		}
		if (strings.Contains(slash, "/assignmentRules/") || strings.Contains(slash, "/queues/")) && family != "assignment_rules" {
			return nil
		}
		raw, err := os.ReadFile(path)
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

func assertPlatformTransactionObservation(t *testing.T, id string, err error, want map[string]any) {
	t.Helper()
	var fault *vm.RuntimeError
	if want["uncaught"] == true {
		if !errors.As(err, &fault) {
			t.Fatalf("native uncaught=%v; Glade=%v", want, err)
		}
		typeName := fault.Type
		if !strings.Contains(typeName, ".") {
			typeName = "System." + typeName
		}
		if typeName != want["exception"] || fault.ExceptionMessage() != want["message"] {
			t.Fatalf("native uncaught=%v; Glade=%v", want, err)
		}
		return
	}
	if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
		t.Fatalf("native observation completed; Glade execution: %v", err)
	}
	assertPlatformTransactionPayload(t, id, fault.ExceptionMessage(), want)
}

func assertPlatformTransactionPayload(t *testing.T, id, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "PLATFORM_OBS|"+id+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing observation transport: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		native, _ := json.Marshal(want)
		t.Fatalf("native=%s; Glade=%s", native, payload)
	}
}

func TestPlatformTransactionPathsNativeConformance(t *testing.T) {
	cases := readPlatformTransactionCases(t)
	for _, family := range platformTransactionFamilies {
		t.Run(family, func(t *testing.T) {
			for _, api := range []string{"62.0", "67.0"} {
				t.Run(api, func(t *testing.T) {
					anonymous := readPlatformTransactionNative(t, family, api, "anonymous", cases)
					named := readPlatformTransactionNative(t, family, api, "istest", cases)
					root := copyPlatformTransactionProject(t, family, api)
					index := loadTestIndex(t, root)
					t.Run("project_semantics", func(t *testing.T) {
						analysis := sema.Analyze(index)
						if index.HasErrors() || analysis.HasErrors() {
							t.Fatalf("native project accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
						}
					})
					selected := Discover(index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}})
					var own []TestCase
					for _, row := range selected {
						if named[row.MethodName] != nil {
							own = append(own, row)
						}
					}
					if len(own) != len(named) {
						t.Fatalf("discovered %d native methods, want %d", len(own), len(named))
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					methods, methodErrors := compileTestMethodsForIndex(&index, own)
					programs, programErrors := compileTestInvokePrograms(own)
					template, err := os.ReadFile(filepath.Join(platformTransactionFixture, "anonymous.apex"))
					if err != nil {
						t.Fatal(err)
					}
					for _, row := range own {
						t.Run(row.MethodName, func(t *testing.T) {
							t.Run("anonymous", func(t *testing.T) {
								source := strings.ReplaceAll(string(template), "CASE_ID", row.MethodName)
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								machine := runner.newMachine()
								machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
								_, err = machine.Execute(program)
								assertPlatformTransactionObservation(t, row.MethodName, err, anonymous[row.MethodName])
							})
							t.Run("named_istest", func(t *testing.T) {
								key := testCaseKey(row)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine := runner.newMachine()
								machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								// Subscriber triggers (CDC rows C02/C03, istest/org.tsv:3-4).
								triggers, triggerErrors := compileProjectTriggers(index)
								if len(triggerErrors) > 0 {
									t.Fatalf("compile triggers: %v", triggerErrors)
								}
								for _, trigger := range triggers {
									if err := machine.RegisterTrigger(trigger); err != nil {
										t.Fatal(err)
									}
								}
								_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
								assertPlatformTransactionObservation(t, row.MethodName, err, named[row.MethodName])
							})
						})
					}
					t.Run("project_test_runner", func(t *testing.T) {
						report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}, NoDiskCache: true, Parallelism: 1}, own)
						results := map[string]testreport.Case{}
						for _, suite := range report.Suites {
							for _, result := range suite.Cases {
								results[result.MethodName] = result
							}
						}
						for _, row := range own {
							t.Run(row.MethodName, func(t *testing.T) {
								result, ok := results[row.MethodName]
								if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
									t.Fatalf("native observation completed; Glade project status=%s; problem=%+v; exclusions=%v", result.Status, result.Problem, report.ExcludedClasses)
								}
								assertPlatformTransactionPayload(t, row.MethodName, result.Problem.Message, named[row.MethodName])
							})
						}
					})
				})
			}
		})
	}
}

func readPlatformCommittedDeliveryNative(t *testing.T, api string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(platformTransactionFixture, "delivery")
	receipt, err := os.ReadFile(filepath.Join(dir, "api"+strings.TrimSuffix(api, ".0"), "native", "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=committed_anonymous\n", "rows=4\n", "exported_rows=3\n", "native_complete=true\n", "deployment_success=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete committed delivery capture: %s", required)
		}
	}
	for _, name := range []string{"enqueue.apex", "readback.apex", "cases.tsv"} {
		source, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		digestKey := "delivery_source_sha256"
		if name == "cases.tsv" {
			digestKey = "exported_delivery_source_sha256"
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("%s:%s=%x\n", digestKey, name, sha256.Sum256(source))) {
			t.Fatalf("native delivery source changed: %s", name)
		}
	}
	verifyPlatformCaptureSetup(t, string(receipt))
	sourceRoot := filepath.Join(platformTransactionFixture, "project")
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		source := strings.ReplaceAll(string(raw), "62.0", api)
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256([]byte(source)))) {
			t.Errorf("native delivery project changed: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "api"+strings.TrimSuffix(api, ".0"), "native", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 4 || lines[0] != "id\troute\tvalue" {
		t.Fatal("incomplete committed delivery rows")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 || cells[1] != "committed_anonymous" || rows[cells[0]] != nil {
			t.Fatalf("invalid committed delivery row: %s", line)
		}
		var row map[string]any
		if json.Unmarshal([]byte(cells[2]), &row) != nil {
			t.Fatal("invalid committed observation")
		}
		rows[cells[0]] = row
	}
	for _, id := range []string{"F07", "F09", "P08"} {
		if rows[id] == nil {
			t.Fatalf("missing committed delivery row %s", id)
		}
	}
	return rows
}

func TestPlatformCommittedDeliveryNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) { readPlatformCommittedDeliveryNative(t, api) })
	}
}

func TestPlatformCommittedDeliveryNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, family := range []string{"flow_paths", "event_flows"} {
			t.Run(api+"/"+family+"/anonymous_transaction_drain", func(t *testing.T) {
				want := readPlatformCommittedDeliveryNative(t, api)
				// The metadata slices have disjoint effects; an unsupported event body
				// must not prevent observing the committed record paths.
				root := copyPlatformTransactionProject(t, family, api)
				index := loadTestIndex(t, root)
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				machine := runner.newMachine()
				execute := func(name string) vm.Result {
					t.Helper()
					source, err := os.ReadFile(filepath.Join(platformTransactionFixture, "delivery", name))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				result := execute("enqueue.apex")
				if err := machine.DrainAsync(&result); err != nil {
					t.Fatal(err)
				}
				result = execute("readback.apex")
				var got map[string]map[string]any
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "PLATFORM_DELIVERY|"); ok {
						if json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatalf("invalid delivery observation: %s", payload)
						}
					}
				}
				if got == nil {
					t.Fatal("missing delivery observation transport")
				}
				ids := []string{"F07", "F09"}
				if family == "event_flows" {
					ids = []string{"P08"}
				}
				for _, id := range ids {
					t.Run(id, func(t *testing.T) {
						if !reflect.DeepEqual(got[id], want[id]) {
							t.Fatalf("native=%v; Glade=%v", want[id], got[id])
						}
					})
				}
			})
		}
	}
}

func verifyPlatformCaptureSetup(t *testing.T, receipt string) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(platformTransactionFixture, "setup", "force-app", "main", "default", "permissionsets", "CaptureFixture.permissionset-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(receipt, "fixture_permission_set_assigned=true\n") || !strings.Contains(receipt, fmt.Sprintf("setup_source_sha256=%x\n", sha256.Sum256(source))) {
		t.Fatal("native fixture permission setup changed")
	}
}
