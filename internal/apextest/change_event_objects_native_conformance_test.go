package apextest

import (
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

const changeEventObjectsFixture = "testdata/conformance/change_event_objects"

func readChangeEventObjectNativeRows(t *testing.T, api, route string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(changeEventObjectsFixture, "native", "api"+strings.TrimSuffix(api, ".0"), route)
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"api=" + api + "\n", "context=" + route + "\n", "rows=2\n", "deployment_success=true\n", "native_complete=true\n"} {
		if !strings.Contains(string(receipt), required) {
			t.Fatalf("incomplete native capture: %s", required)
		}
	}
	err = filepath.WalkDir(filepath.Join(changeEventObjectsFixture, "project"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(filepath.Join(changeEventObjectsFixture, "project"), path)
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
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 3 || lines[0] != "id\troute\tobservation" {
		t.Fatal("incomplete native observations")
	}
	rows := map[string]map[string]any{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 || cells[1] != route || rows[cells[0]] != nil {
			t.Fatalf("invalid native row: %s", line)
		}
		var value map[string]any
		if json.Unmarshal([]byte(cells[2]), &value) != nil || len(value) != 10 {
			t.Fatal("invalid native payload")
		}
		rows[cells[0]] = value
	}
	if rows["accountFields"]["name"] != "AccountChangeEvent" || rows["customFields"]["name"] != "Parcel__ChangeEvent" {
		t.Fatal("missing standard/custom native rows")
	}
	cases, err := os.ReadFile(filepath.Join(changeEventObjectsFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	caseLines := strings.Split(strings.TrimSpace(string(cases)), "\n")
	if len(caseLines) != 3 || caseLines[0] != "id\tobject\tnative_scope\troutes\tapis" {
		t.Fatal("incomplete native case table")
	}
	for _, line := range caseLines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 5 || rows[cells[0]]["name"] != cells[1] || cells[2] == "" || cells[3] != "anonymous,istest" || cells[4] != "62.0,67.0" {
			t.Fatalf("invalid native case: %s", line)
		}
		if route == "anonymous" {
			source, err := os.ReadFile(filepath.Join(changeEventObjectsFixture, "anonymous", cells[0]+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(receipt), fmt.Sprintf("input_sha256:%s.apex=%x\n", cells[0], sha256.Sum256(source))) {
				t.Fatalf("native anonymous input changed: %s", cells[0])
			}
		}
	}
	return rows
}

func TestChangeEventObjectNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) { readChangeEventObjectNativeRows(t, api, route) })
		}
	}
}

func copyChangeEventObjectProject(t *testing.T, api, method string) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(changeEventObjectsFixture, "project")
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if strings.Contains(filepath.ToSlash(name), "/triggers/") {
			if method == "accountFields" && strings.HasPrefix(filepath.Base(path), "ParcelChangeShape.") {
				return nil
			}
			if method == "customFields" && strings.HasPrefix(filepath.Base(path), "AccountChangeShape.") {
				return nil
			}
		}
		writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func assertChangeEventObjectObservation(t *testing.T, method, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "OBSERVED|"+method+"|")
	var got map[string]any
	if !ok || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing native observation transport: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%v; Glade=%v", want, got)
	}
}

func TestChangeEventProjectTriggerCompileRegisterAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, method := range []string{"accountFields", "customFields"} {
			t.Run(api+"/"+method, func(t *testing.T) {
				rows := readChangeEventObjectNativeRows(t, api, "istest")
				index := loadTestIndex(t, copyChangeEventObjectProject(t, api, method))
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native trigger compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				for _, route := range []string{"anonymous", "istest"} {
					t.Run(route, func(t *testing.T) {
						machine := runner.newMachine()
						if route == "istest" {
							machine.EnableTestContext()
						}
						object := rows[method]["name"].(string)
						if len(machine.Triggers[object]) != 1 {
							t.Fatalf("native trigger on %s compiled; registered triggers=%v", object, machine.Triggers)
						}
					})
				}
			})
		}
	}
}

func TestChangeEventObjectsNativeConformanceAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, method := range []string{"accountFields", "customFields"} {
			t.Run(api+"/"+method, func(t *testing.T) {
				index := loadTestIndex(t, copyChangeEventObjectProject(t, api, method))
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native project compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				selected := Discover(index, Options{SelectedClasses: []string{"ChangeShapeProbeTest"}, SelectedMethod: method})
				if len(selected) != 1 {
					t.Fatalf("expected one named native route, found %d", len(selected))
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, selected)
				programs, programErrors := compileTestInvokePrograms(selected)
				for _, route := range []string{"anonymous", "istest"} {
					t.Run(route, func(t *testing.T) {
						rows := readChangeEventObjectNativeRows(t, api, route)
						machine := runner.newMachine()
						var err error
						if route == "istest" {
							row := selected[0]
							key := testCaseKey(row)
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
							}
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, err = machine.ExecuteTestInClass(programs[key], row.ClassName)
						} else {
							source, readErr := os.ReadFile(filepath.Join(changeEventObjectsFixture, "anonymous", method+".apex"))
							if readErr != nil {
								t.Fatal(readErr)
							}
							program, compileErr := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
							if compileErr != nil {
								t.Fatal(compileErr)
							}
							_, err = machine.Execute(program)
						}
						var fault *vm.RuntimeError
						if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
							t.Fatalf("native observation completed; Glade=%v", err)
						}
						assertChangeEventObjectObservation(t, method, fault.ExceptionMessage(), rows[method])
					})
				}
				t.Run("project_istest", func(t *testing.T) {
					rows := readChangeEventObjectNativeRows(t, api, "istest")
					run := Run(index, Options{SelectedClasses: []string{"ChangeShapeProbeTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected one synthetic project case: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("native observation completed; Glade status=%s problem=%+v", result.Status, result.Problem)
					}
					assertChangeEventObjectObservation(t, method, result.Problem.Message, rows[method])
				})
			})
		}
	}
}
