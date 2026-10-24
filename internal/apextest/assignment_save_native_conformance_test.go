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

const assignmentSaveFixture = "testdata/conformance/assignment_save_routes"

func readAssignmentSaveNative(t *testing.T, fixture, api, route string) map[string]map[string]any {
	t.Helper()
	base := filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), route)
	raw, err := os.ReadFile(filepath.Join(base, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	note, err := os.ReadFile(filepath.Join(base, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=" + route + "\n", "native_complete=true\n", "deployment_success=true\n", fmt.Sprintf("org_tsv_sha256=%x\n", sha256.Sum256(raw))} {
		if !strings.Contains(string(note), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	if err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256([]byte(strings.ReplaceAll(string(raw), "62.0", api)))
		if !strings.Contains(string(note), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), digest)) {
			t.Errorf("native source changed: %s", rel)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cases.tsv", "anonymous.apex"} {
		source, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(note), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256(source))) {
			t.Fatalf("native source changed: %s", name)
		}
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 || cells[1] != api || cells[2] != route || rows[cells[0]] != nil {
			t.Fatalf("invalid native row %q", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[3]), &value) != nil {
			t.Fatalf("invalid native payload %s", cells[0])
		}
		rows[cells[0]] = value
	}
	table, err := os.ReadFile(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	cases := strings.Split(strings.TrimSpace(string(table)), "\n")[1:]
	if len(rows) != len(cases) {
		t.Fatalf("native rows=%d; cases=%d", len(rows), len(cases))
	}
	for _, row := range cases {
		id, _, _ := strings.Cut(row, "\t")
		if rows[id] == nil {
			t.Fatalf("missing native %s", id)
		}
	}
	return rows
}

func assertAssignmentSavePayload(t *testing.T, id, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "ASSIGN_SAVE|"+id+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing observation: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%v; project=%v", want, got)
	}
}

func TestAssignmentSaveNativeConformanceAPIRoutes(t *testing.T) {
	testAssignmentSaveNativeAPIRoutes(t, assignmentSaveFixture)
}

func TestAssignmentSaveScalarAndFilteredNativeConformanceAPIRoutes(t *testing.T) {
	testAssignmentSaveNativeAPIRoutes(t, "testdata/conformance/assignment_save_edge_routes")
}

func TestAssignmentSaveUserModeNativeConformanceAPIRoutes(t *testing.T) {
	testAssignmentSaveNativeAPIRoutes(t, "testdata/conformance/assignment_save_user_routes")
}

func testAssignmentSaveNativeAPIRoutes(t *testing.T, fixture string, routes ...string) {
	t.Helper()
	if len(routes) == 0 {
		routes = []string{"anonymous", "named_istest"}
	}
	restoreDisk := EnableDiskCacheForTesting()
	t.Cleanup(restoreDisk)
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			native := map[string]map[string]map[string]any{}
			for _, route := range routes {
				nativeRoute := route
				if route == "named_istest" {
					nativeRoute = "istest"
				}
				native[nativeRoute] = readAssignmentSaveNative(t, fixture, api, nativeRoute)
			}
			root := t.TempDir()
			sourceRoot := filepath.Join(fixture, "project")
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
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(raw), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) != 0 {
				t.Fatal(triggerErrors)
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			own := Discover(index, Options{SelectedClasses: []string{"AssignmentSaveObservationTest"}})
			if len(own) != len(native["istest"]) {
				t.Fatalf("discovered=%d; native=%d", len(own), len(native["istest"]))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, own)
			programs, programErrors := compileTestInvokePrograms(own)
			for _, row := range own {
				for _, route := range routes {
					t.Run(route+"/"+row.MethodName, func(t *testing.T) {
						machine := runner.newMachine()
						machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
						var err error
						nativeRoute := "anonymous"
						if route == "anonymous" {
							program, compileErr := vm.CompileAnonymousWithOptions("System.assert(false, 'ASSIGN_SAVE|"+row.MethodName+"|' + AssignmentSaveProbe.run('"+row.MethodName+"'));", vm.CompileOptions{APIVersion: api})
							if compileErr != nil {
								t.Fatal(compileErr)
							}
							_, err = machine.Execute(program)
						} else {
							nativeRoute = "istest"
							key := testCaseKey(row)
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
							}
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, err = machine.ExecuteTestInClass(programs[key], row.ClassName)
						}
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("native observation completed; project=%v", err)
						}
						assertAssignmentSavePayload(t, row.MethodName, fault.ExceptionMessage(), native[nativeRoute][row.MethodName])
					})
				}
			}
			for _, route := range []struct {
				name     string
				parallel int
				cache    bool
			}{{"runner_serial", 1, false}, {"runner_parallel", 2, false}, {"runner_cache_cold", 1, true}, {"runner_cache_restored", 1, true}, {"runner_cache_restored_parallel", 2, true}} {
				t.Run(route.name, func(t *testing.T) {
					InvalidateRuntimeCaches()
					ResetPerfCounters()
					report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"AssignmentSaveObservationTest"}, NoDiskCache: !route.cache, Parallelism: route.parallel, ParallelMethods: route.parallel > 1, RestoredRuntimeMultiWorker: route.name == "runner_cache_restored_parallel", PerfCounters: true}, own)
					if strings.HasPrefix(route.name, "runner_cache_restored") && SnapshotPerfCounters().Phases.DiskCacheHits != 1 {
						t.Errorf("restored route did not use disk runtime: %+v", SnapshotPerfCounters().Phases)
					}
					results := map[string]testreport.Case{}
					for _, suite := range report.Suites {
						for _, row := range suite.Cases {
							results[row.MethodName] = row
						}
					}
					for _, row := range own {
						t.Run(row.MethodName, func(t *testing.T) {
							result, ok := results[row.MethodName]
							if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing native observation; result=%+v exclusions=%v", result, report.ExcludedClasses)
							}
							assertAssignmentSavePayload(t, row.MethodName, result.Problem.Message, native["istest"][row.MethodName])
						})
					}
				})
			}
		})
	}
}
