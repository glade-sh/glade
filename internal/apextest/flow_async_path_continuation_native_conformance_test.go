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

	"github.com/glade-sh/glade/internal/vm"
)

const flowAsyncContinuationFixture = "testdata/conformance/flow_async_path_continuations"

func readFlowAsyncContinuationNative(t *testing.T, api, route string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(flowAsyncContinuationFixture, "native", "api"+strings.TrimSuffix(api, ".0"), route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"api=" + api + "\n", "context=" + route + "\n", "rows=2\n", "deployment_success=true\n", "native_complete=true\n"} {
		if !strings.Contains(string(receipt), value) {
			t.Fatalf("incomplete native capture: %s", value)
		}
	}
	root := filepath.Join(flowAsyncContinuationFixture, "project")
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256([]byte(strings.ReplaceAll(string(body), "62.0", api))))) {
			t.Errorf("native source changed: %s", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cases.tsv", "execute.apex", "seed.apex", "rollback.apex", "readback.apex"} {
		body, err := os.ReadFile(filepath.Join(flowAsyncContinuationFixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("input_sha256:%s=%x\n", name, sha256.Sum256(body))) {
			t.Fatalf("native input changed: %s", name)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 3 || lines[0] != "id\troute\tobservation" {
		t.Fatal("incomplete native rows")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 || cells[1] != route || rows[cells[0]] != nil {
			t.Fatal("invalid native row")
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[2]), &value) != nil || len(value) != 3 {
			t.Fatal("invalid native observation")
		}
		rows[cells[0]] = value
	}
	if rows["EXECUTE"] == nil || rows["ROLLBACK"] == nil {
		t.Fatal("missing native route")
	}
	return rows
}

func TestFlowAsyncPathContinuationNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"committed_anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) { readFlowAsyncContinuationNative(t, api, route) })
		}
	}
}

func TestFlowAsyncPathContinuationNativeConformanceAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(flowAsyncContinuationFixture, "project")
			err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(sourceRoot, path)
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
			selected := Discover(index, Options{SelectedClasses: []string{"AsyncContinuationProbeTest"}})
			if len(selected) != 2 {
				t.Fatalf("discovered %d methods; want 2", len(selected))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(runtime.Triggers) != 1 {
				t.Fatalf("compiled triggers=%d; want 1", len(runtime.Triggers))
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			execute := func(t *testing.T, machine *vm.VM, name string) vm.Result {
				t.Helper()
				source, err := os.ReadFile(filepath.Join(flowAsyncContinuationFixture, name))
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
			committed := readFlowAsyncContinuationNative(t, api, "committed_anonymous")
			named := readFlowAsyncContinuationNative(t, api, "istest")
			for _, key := range []string{"EXECUTE", "ROLLBACK"} {
				t.Run(key+"/committed_anonymous", func(t *testing.T) {
					machine := runner.newMachine()
					name := "execute.apex"
					if key == "ROLLBACK" {
						execute(t, machine, "seed.apex")
						name = "rollback.apex"
					}
					result := execute(t, machine, name)
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					result = execute(t, machine, "readback.apex")
					for _, line := range result.Debug {
						if _, payload, ok := strings.Cut(line, "CONTINUATION|"); ok {
							var got map[string]any
							if json.Unmarshal([]byte(payload), &got) != nil {
								t.Fatal("invalid observation")
							}
							if !reflect.DeepEqual(got, committed[key]) {
								t.Fatalf("native=%v; Glade=%v", committed[key], got)
							}
							return
						}
					}
					t.Fatal("missing committed observation")
				})
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, selected)
			programs, programErrors := compileTestInvokePrograms(selected)
			for _, row := range selected {
				t.Run(row.MethodName+"/named_istest", func(t *testing.T) {
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
					var failure *vm.RuntimeError
					if !errors.As(err, &failure) || !strings.HasSuffix(failure.Type, "AssertException") {
						t.Fatalf("native result assertion; Glade=%v", err)
					}
					assertFlowAsyncContinuation(t, row.MethodName, failure.Message, named[row.MethodName])
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"AsyncContinuationProbeTest"}, NoDiskCache: true, Parallelism: 1}, selected)
				count := 0
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						count++
						t.Run(row.MethodName, func(t *testing.T) {
							if row.Problem == nil || !strings.HasSuffix(row.Problem.Type, "AssertException") {
								t.Fatalf("native assertion; status=%s problem=%v", row.Status, row.Problem)
							}
							assertFlowAsyncContinuation(t, row.MethodName, row.Problem.Message, named[row.MethodName])
						})
					}
				}
				if count != 2 {
					t.Fatalf("observations=%d; want 2", count)
				}
			})
		})
	}
}

func assertFlowAsyncContinuation(t *testing.T, key, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "CONTINUATION|"+key+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing observation: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%v; Glade=%v", want, got)
	}
}
