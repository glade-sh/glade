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
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

const eventElementsFixture = "testdata/conformance/platform_event_flow_elements"

func TestPlatformEventElementsNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			rows := eventElementsRows(t, api)
			eventElementsProject(t, api)
			raw, err := os.ReadFile(filepath.Join(eventElementsFixture, "cases.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			cases := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(cases) != 11 || cases[0] != "id\toperation\tshape\troutes\tapis" {
				t.Fatal("incomplete event case table")
			}
			for _, line := range cases[1:] {
				cells := strings.Split(line, "\t")
				if len(cells) != 5 || cells[3] != "anonymous,deliver,stop" || cells[4] != "62.0,67.0" {
					t.Fatalf("invalid event case: %s", line)
				}
				for _, route := range []string{"anonymous", "deliver", "stop"} {
					if rows[cells[0]+"/"+route] == nil {
						t.Fatalf("missing native route: %s/%s", cells[0], route)
					}
				}
			}
		})
	}
}

func eventElementsRows(t *testing.T, api string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(eventElementsFixture, "native", "api"+strings.TrimSuffix(api, ".0"))
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 31 || lines[0] != "id\troute\tvalue" {
		t.Fatal("incomplete native event route matrix")
	}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		var value map[string]any
		if len(cells) != 3 || json.Unmarshal([]byte(cells[2]), &value) != nil || value == nil || rows[cells[0]+"/"+cells[1]] != nil {
			t.Fatalf("invalid native event row: %s", line)
		}
		rows[cells[0]+"/"+cells[1]] = value
	}
	return rows
}

func eventElementsProject(t *testing.T, api string) string {
	t.Helper()
	dir := filepath.Join(eventElementsFixture, "native", "api"+strings.TrimSuffix(api, ".0"))
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"api=" + api + "\n", "native_complete=true\n", "deployment_success=true\n", "rows=20\n", "anonymous_rows=10\n"} {
		if !strings.Contains(string(receipt), field) {
			t.Fatalf("incomplete native capture: %s", field)
		}
	}
	for _, name := range []string{"cases.tsv", "anonymous-seed.apex", "anonymous-publish.apex", "anonymous-read.apex"} {
		source, err := os.ReadFile(filepath.Join(eventElementsFixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("input_sha256:%s=%x\n", name, sha256.Sum256(source))) {
			t.Fatalf("native input changed: %s", name)
		}
	}
	sourceRoot := filepath.Join(eventElementsFixture, "project")
	root := t.TempDir()
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
		writeFile(t, filepath.Join(root, rel), source)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return root
}

func compareEventElements(t *testing.T, payload string, want map[string]any) {
	t.Helper()
	var got map[string]any
	if want == nil || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing event observation: %s", payload)
	}
	// Subscriber order is not an ordering contract. Preserve the captured order
	// in the native rows, while asserting every subscriber's effects.
	normalize := func(row map[string]any) map[string]any {
		copy := make(map[string]any, len(row))
		for key, value := range row {
			copy[key] = value
		}
		if values, ok := row["phases"].([]any); ok {
			phases := make([]string, len(values))
			for i, value := range values {
				phases[i] = fmt.Sprint(value)
			}
			sort.Strings(phases)
			copy["phases"] = phases
		}
		return copy
	}
	if !reflect.DeepEqual(normalize(got), normalize(want)) {
		t.Fatalf("native=%v; Glade=%v", want, got)
	}
}

func TestPlatformEventElementsNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := eventElementsRows(t, api)
			index := loadTestIndex(t, eventElementsProject(t, api))
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"ElementProbeTest"}})
			if len(cases) != 20 {
				t.Fatalf("discovered %d methods, want 20", len(cases))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			triggers, triggerErrors := compileProjectTriggers(index)
			if len(triggerErrors) != 0 || len(triggers) != 1 {
				t.Fatalf("native trigger accepted: %v; triggers=%d", triggerErrors, len(triggers))
			}
			for _, trigger := range triggers {
				if err := runner.base.RegisterTrigger(trigger); err != nil {
					t.Fatal(err)
				}
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, row := range cases {
				t.Run("named_istest/"+row.MethodName, func(t *testing.T) {
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
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("native observation completed; Glade: %v; subscriber rows: %+v", err, machine.Org.Objects["ElementLog__c"].Records)
					}
					parts := strings.SplitN(row.MethodName, "_", 2)
					prefix := "ELEMENT_OBS|" + parts[0] + "|" + parts[1] + "|"
					_, payload, ok := strings.Cut(fault.ExceptionMessage(), prefix)
					if !ok {
						t.Fatal(fault.ExceptionMessage())
					}
					compareEventElements(t, payload, want[parts[0]+"/"+parts[1]])
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"ElementProbeTest"}, NoDiskCache: true, Parallelism: 1}, cases)
				results := map[string]testreport.Case{}
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						results[row.MethodName] = row
					}
				}
				for _, row := range cases {
					t.Run(row.MethodName, func(t *testing.T) {
						result, ok := results[row.MethodName]
						if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("project status=%s; problem=%+v; exclusions=%v", result.Status, result.Problem, report.ExcludedClasses)
						}
						parts := strings.SplitN(row.MethodName, "_", 2)
						_, payload, ok := strings.Cut(result.Problem.Message, "ELEMENT_OBS|"+parts[0]+"|"+parts[1]+"|")
						if !ok {
							t.Fatal(result.Problem.Message)
						}
						compareEventElements(t, payload, want[parts[0]+"/"+parts[1]])
					})
				}
			})
			t.Run("anonymous_committed", func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join(eventElementsFixture, "cases.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					cells := strings.Split(line, "\t")
					t.Run(cells[0], func(t *testing.T) {
						machine := runner.newMachine()
						execute := func(name string) vm.Result {
							t.Helper()
							source, err := os.ReadFile(filepath.Join(eventElementsFixture, name))
							if err != nil {
								t.Fatal(err)
							}
							body := strings.NewReplacer("CASE_ID", cells[0], "OPERATION", cells[1]).Replace(string(source))
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatal(err)
							}
							return result
						}
						execute("anonymous-seed.apex")
						result := execute("anonymous-publish.apex")
						if err := machine.DrainAsync(&result); err != nil {
							t.Fatal(err)
						}
						result = execute("anonymous-read.apex")
						for _, line := range result.Debug {
							if _, payload, ok := strings.Cut(line, "ELEMENT_OBS|"+cells[0]+"|anonymous|"); ok {
								compareEventElements(t, payload, want[cells[0]+"/anonymous"])
								return
							}
						}
						t.Fatal("missing anonymous event observation")
					})
				}
			})
		})
	}
}
