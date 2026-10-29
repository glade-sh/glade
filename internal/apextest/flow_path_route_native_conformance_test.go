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
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const flowPathRouteFixture = platformTransactionFixture + "/flow_path_routes"

func readFlowPathRouteNative(t *testing.T, api, route string) map[string]map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(flowPathRouteFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 17 || lines[0] != "id\tshape\troutes\tapis" {
		t.Fatal("incomplete route case table")
	}
	dir := filepath.Join(flowPathRouteFixture, "native", "api"+strings.TrimSuffix(api, ".0"), route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=" + route + "\n", "rows=16\n", "native_complete=true\n", "deployment_success=true\n", "fixture_permission_set_assigned=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("missing capture field %s", required)
		}
	}
	err = filepath.WalkDir(flowPathRouteFixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(flowPathRouteFixture, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), "native/") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.ToSlash(rel), "project/") {
			body = []byte(strings.ReplaceAll(string(body), "62.0", api))
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256(body))) {
			t.Errorf("native source changed: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	nativeLines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if nativeLines[0] != "id\tgroup\troute\tstatus\tvalue" {
		t.Fatal("invalid native header")
	}
	rows := map[string]map[string]any{}
	for _, line := range nativeLines[1:] {
		cells := strings.SplitN(line, "\t", 5)
		if len(cells) != 5 || cells[1] != "flow_path_routes" || cells[2] != route || cells[3] != "runtime" || rows[cells[0]] != nil {
			t.Fatalf("invalid native row: %s", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[4]), &value) != nil || value == nil {
			t.Fatal("invalid native value")
		}
		rows[cells[0]] = value
	}
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 4 || cells[1] == "" || cells[2] != "anonymous,istest,project" || cells[3] != "62.0,67.0" || rows[cells[0]] == nil {
			t.Fatalf("invalid or missing case: %s", line)
		}
	}
	if len(rows) != 16 {
		t.Fatalf("native rows=%d", len(rows))
	}
	return rows
}

func TestFlowPathRouteNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) { readFlowPathRouteNative(t, api, route) })
		}
	}
}

func TestFlowPathRouteNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous := readFlowPathRouteNative(t, api, "anonymous")
			named := readFlowPathRouteNative(t, api, "istest")
			root := t.TempDir()
			source := filepath.Join(flowPathRouteFixture, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native deployment accepted; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			own := Discover(index, Options{SelectedClasses: []string{"FlowRouteObservationTest"}})
			if len(own) != len(named) {
				t.Fatalf("discovered=%d native=%d", len(own), len(named))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethodsForIndex(&index, own)
			programs, programErrors := compileTestInvokePrograms(own)
			template, err := os.ReadFile(filepath.Join(flowPathRouteFixture, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range own {
				t.Run(row.MethodName, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(strings.ReplaceAll(string(template), "CASE_ID", row.MethodName), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						assertFlowPathRouteObservation(t, row.MethodName, err, anonymous[row.MethodName])
					})
					t.Run("named_istest", func(t *testing.T) {
						key := testCaseKey(row)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
						assertFlowPathRouteObservation(t, row.MethodName, err, named[row.MethodName])
					})
				})
			}
			// Exercise built, memory-cached and disk-restored project runtimes.
			for _, state := range []string{"built", "memory", "disk"} {
				t.Run("project_test_runner/"+state, func(t *testing.T) {
					if state != "memory" {
						InvalidateRuntimeCaches()
					}
					ResetPerfCounters()
					report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"FlowRouteObservationTest"}, NoDiskCache: state != "disk", Parallelism: 1, PerfCounters: true}, own)
					seen := map[string]bool{}
					for _, suite := range report.Suites {
						for _, result := range suite.Cases {
							t.Run(result.MethodName, func(t *testing.T) {
								if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
									t.Fatalf("native observation completed; status=%s problem=%+v", result.Status, result.Problem)
								}
								assertPlatformTransactionPayload(t, result.MethodName, strings.Replace(result.Problem.Message, "FLOW_ROUTE|", "PLATFORM_OBS|", 1), named[result.MethodName])
								seen[result.MethodName] = true
							})
						}
					}
					if len(seen) != len(own) {
						t.Fatalf("observed=%d want=%d", len(seen), len(own))
					}
					phases := SnapshotPerfCounters().Phases
					if state == "memory" && phases.MemoryCacheHits != 1 {
						t.Fatalf("missing memory cache hit: %+v", phases)
					}
					if state == "disk" && phases.DiskCacheHits != 1 {
						t.Fatalf("missing disk cache hit: %+v", phases)
					}
					if state == "built" {
						key, entry, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache(), false)
						if err != nil {
							t.Fatal(err)
						}
						persistDiskRuntimeWithPerf(index, nil, key, entry.restored.CloneOrg(), entry, newRunPerfCounters(true))
					}
				})
			}
		})
	}
}

func assertFlowPathRouteObservation(t *testing.T, id string, err error, want map[string]any) {
	t.Helper()
	if want["uncaught"] == true {
		assertPlatformTransactionObservation(t, id, err, want)
		return
	}
	var fault *vm.RuntimeError
	if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
		t.Fatalf("native observation completed; execution=%v", err)
	}
	assertPlatformTransactionPayload(t, id, strings.Replace(fault.ExceptionMessage(), "FLOW_ROUTE|", "PLATFORM_OBS|", 1), want)
}
