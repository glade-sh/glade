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
	"github.com/glade-sh/glade/internal/vm"
)

const flowAsyncPathFixture = "testdata/conformance/flow_async_paths"

func readFlowAsyncPathNative(t *testing.T, api, route string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(flowAsyncPathFixture, "native", "api"+strings.TrimSuffix(api, ".0"), route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=" + route + "\n", "deployment_success=true\n", "native_complete=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	sourceRoot := filepath.Join(flowAsyncPathFixture, "project")
	err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
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
		body = []byte(strings.ReplaceAll(string(body), "62.0", api))
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
			t.Errorf("native source changed: %s", name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cases.tsv", "anonymous.apex", "enqueue.apex", "readback.apex"} {
		body, err := os.ReadFile(filepath.Join(flowAsyncPathFixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("input_sha256:%s=%x\n", name, sha256.Sum256(body))) {
			t.Errorf("native input changed: %s", name)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 || lines[0] != "id\troute\tobservation" {
		t.Fatal("invalid native rows")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 || cells[1] != route || rows[cells[0]] != nil {
			t.Fatalf("invalid native row: %s", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[2]), &value) != nil || value == nil {
			t.Fatal("invalid native JSON")
		}
		rows[cells[0]] = value
	}
	cases, err := os.ReadFile(filepath.Join(flowAsyncPathFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	caseLines := strings.Split(strings.TrimSpace(string(cases)), "\n")
	if len(caseLines) != 10 || caseLines[0] != "id\tshape\troutes\tapis" {
		t.Fatal("incomplete case table")
	}
	for _, line := range caseLines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 4 || cells[1] == "" || cells[2] != "anonymous,istest" || cells[3] != "62.0,67.0" {
			t.Fatalf("invalid case: %s", line)
		}
		if route != "committed_anonymous" && rows[cells[0]] == nil {
			t.Fatalf("missing native case %s", cells[0])
		}
	}
	wantCount := len(caseLines) - 1
	if route == "committed_anonymous" {
		wantCount = 1
		if rows["COMMIT"] == nil {
			t.Fatal("missing committed row")
		}
	}
	if len(rows) != wantCount || !strings.Contains(string(receipt), fmt.Sprintf("rows=%d\n", wantCount)) {
		t.Fatal("unexpected native row count")
	}
	return rows
}

func TestFlowAsyncPathNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest", "committed_anonymous"} {
			t.Run(api+"/"+route, func(t *testing.T) { readFlowAsyncPathNative(t, api, route) })
		}
	}
}

func copyFlowAsyncPathProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	sourceRoot := filepath.Join(flowAsyncPathFixture, "project")
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
	return root
}

func assertFlowAsyncPathObservation(t *testing.T, key, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "ASYNC_PATH|"+key+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing native observation: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%v; Glade=%v", want, got)
	}
}

func TestFlowAsyncPathNativeConformanceAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			anonymous := readFlowAsyncPathNative(t, api, "anonymous")
			named := readFlowAsyncPathNative(t, api, "istest")
			index := loadTestIndex(t, copyFlowAsyncPathProject(t, api))
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			selected := Discover(index, Options{SelectedClasses: []string{"AsyncPathProbeTest"}})
			if len(selected) != len(named) {
				t.Fatalf("discovered %d methods; want %d", len(selected), len(named))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethodsForIndex(&index, selected)
			programs, programErrors := compileTestInvokePrograms(selected)
			template, err := os.ReadFile(filepath.Join(flowAsyncPathFixture, "anonymous.apex"))
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range selected {
				t.Run(row.MethodName, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions(strings.ReplaceAll(string(template), "CASE_ID", row.MethodName), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.newMachine().Execute(program)
						if anonymous[row.MethodName]["uncaught"] == true {
							var runtimeErr *vm.RuntimeError
							if !errors.As(err, &runtimeErr) || runtimeErr.Type != anonymous[row.MethodName]["exception"] || runtimeErr.Message != anonymous[row.MethodName]["message"] {
								t.Fatalf("native=%v; Glade=%v", anonymous[row.MethodName], err)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
						for _, line := range result.Debug {
							if strings.Contains(line, "ASYNC_PATH|"+row.MethodName+"|") {
								assertFlowAsyncPathObservation(t, row.MethodName, line, anonymous[row.MethodName])
								return
							}
						}
						t.Fatal("missing anonymous observation")
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
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
							t.Fatalf("native result assertion; Glade=%v", err)
						}
						assertFlowAsyncPathObservation(t, row.MethodName, runtimeErr.Message, named[row.MethodName])
					})
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"AsyncPathProbeTest"}, NoDiskCache: true, Parallelism: 1}, selected)
				observed := 0
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						observed++
						t.Run(row.MethodName, func(t *testing.T) {
							if row.Problem == nil || !strings.HasSuffix(row.Problem.Type, "AssertException") {
								t.Fatalf("native result assertion; status=%s problem=%+v", row.Status, row.Problem)
							}
							assertFlowAsyncPathObservation(t, row.MethodName, row.Problem.Message, named[row.MethodName])
						})
					}
				}
				if observed != len(selected) {
					t.Fatalf("project observations=%d; want %d", observed, len(selected))
				}
			})
			t.Run("anonymous_committed_transaction_drain", func(t *testing.T) {
				want := readFlowAsyncPathNative(t, api, "committed_anonymous")["COMMIT"]
				machine := runner.newMachine()
				execute := func(name string) vm.Result {
					t.Helper()
					source, err := os.ReadFile(filepath.Join(flowAsyncPathFixture, name))
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
				found := false
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "ASYNC_BEFORE|"); ok {
						found = true
						if payload != fmt.Sprint(want["before"]) {
							t.Errorf("before commit: native=%v; Glade=%s", want["before"], payload)
						}
					}
				}
				if !found {
					t.Fatal("missing before commit observation")
				}
				if err := machine.DrainAsync(&result); err != nil {
					t.Fatal(err)
				}
				result = execute("readback.apex")
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "ASYNC_COMMITTED|"); ok {
						var got map[string]any
						if json.Unmarshal([]byte(payload), &got) != nil {
							t.Fatal("invalid committed observation")
						}
						for key, value := range want {
							if key != "before" && !reflect.DeepEqual(got[key], value) {
								t.Errorf("%s: native=%v; Glade=%v", key, value, got[key])
							}
						}
						return
					}
				}
				t.Fatal("missing committed observation")
			})
		})
	}
}
