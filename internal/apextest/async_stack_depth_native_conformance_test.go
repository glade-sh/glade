package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These rows distinguish stopTest flushing from actual standalone callbacks.
// Caught enqueue exceptions and uncaught job failures are separate observations.
func TestAsyncStackDepthAnonymousAndStopTestNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "async_stack_depth")
	readTSV := func(path string) [][]string {
		t.Helper()
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	cases := readTSV(filepath.Join(dir, "cases.tsv"))
	if len(cases) != 23 {
		t.Fatalf("native case table has %d cases, want 22", len(cases)-1)
	}
	caseNames := make(map[string]bool)
	for _, row := range cases[1:] {
		if caseNames[row[0]] {
			t.Fatalf("duplicate case %q", row[0])
		}
		caseNames[row[0]] = true
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(source), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			rows := readTSV(filepath.Join(dir, "api"+api[:2], "org.tsv"))
			if len(rows) != 45 || strings.Join(rows[0], ",") != "source_api,route,case,outcome,exception_type,message,observations" {
				t.Fatal("incomplete native stack-depth matrix")
			}
			seen := make(map[string]bool)
			for _, row := range rows[1:] {
				key := row[1] + "/" + row[2]
				if row[0] != api || !caseNames[row[2]] || seen[key] || (row[1] != "anonymous" && row[1] != "istest") {
					t.Fatalf("invalid native row %v", row[:3])
				}
				seen[key] = true
				t.Run(key, func(t *testing.T) {
					machine := runner.newMachine()
					execute := func(source string) (vm.Result, error) {
						t.Helper()
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						return machine.Execute(program)
					}
					outcome, exceptionType, message := "observed", "", ""
					values := []string{}
					if row[1] == "istest" {
						selected := Discover(index, Options{SelectedClasses: []string{"StackDepthProbeTest"}, SelectedMethod: row[2]})
						if len(selected) != 1 {
							t.Fatalf("named route selection: %v", selected)
						}
						methods, methodErrors := compileTestMethodsForIndex(&index, selected)
						programs, programErrors := compileTestInvokePrograms(selected)
						key := testCaseKey(selected[0])
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named route compile: %v %v", methodErrors[key], programErrors[key])
						}
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, err := machine.ExecuteInClass(programs[key], selected[0].ClassName)
						if err == nil {
							t.Fatal("named route produced neither observation nor exception")
						}
						exceptionType, message = asyncStackDepthFaultDetails(err)
						if payload, ok := strings.CutPrefix(message, "Assertion Failed: STACK|"); ok && strings.HasSuffix(exceptionType, "AssertException") {
							if err := json.Unmarshal([]byte(payload), &values); err != nil {
								t.Fatal(err)
							}
							exceptionType, message = "", ""
						} else {
							outcome = "thrown"
						}
					} else {
						source, err := os.ReadFile(filepath.Join(dir, "anonymous", row[2]+".apex"))
						if err != nil {
							t.Fatal(err)
						}
						result, err := execute(string(source))
						if err != nil {
							outcome = "thrown"
							exceptionType, message = asyncStackDepthFaultDetails(err)
						} else if err := machine.DrainAsync(&result); err != nil {
							outcome = "job-error"
							exceptionType, message = asyncStackDepthFaultDetails(err)
						}
						readback, err := execute("System.debug('STACK|'+JSON.serialize(StackDepthProbe.readback('" + row[2] + "')));")
						if err != nil {
							t.Fatal(err)
						}
						found := false
						for _, line := range readback.Debug {
							if _, payload, ok := strings.Cut(line, "STACK|"); ok {
								if found {
									t.Fatal("duplicate readback")
								}
								found = true
								if err := json.Unmarshal([]byte(payload), &values); err != nil {
									t.Fatal(err)
								}
							}
						}
						if !found {
							t.Fatal("missing standalone readback")
						}
					}
					var wantValues []string
					if err := json.Unmarshal([]byte(row[6]), &wantValues); err != nil {
						t.Fatal(err)
					}
					typeDiffers := !asyncStackDepthTypeMatches(outcome, exceptionType, row[4])
					if outcome != row[3] || typeDiffers || message != row[5] || !reflect.DeepEqual(values, wantValues) {
						t.Fatalf("native=(%s,%q,%q,%s); Glade=(%s,%q,%q,%v)", row[3], row[4], row[5], row[6], outcome, exceptionType, message, values)
					}
				})
			}
		})
	}
}

// Native type expectations: async_stack_depth/api{62,67}/org.tsv:19,23,37,45;
// adjacent ORG-RUN.txt records the uncaught standalone fatal-log capture.
func asyncStackDepthFaultDetails(err error) (string, string) {
	var fault *vm.RuntimeError
	if errors.As(err, &fault) {
		return fault.Type, fault.ExceptionMessage()
	}
	// DrainAsync exposes vm.apexThrowError without a public value accessor.
	// Read only its actual vm.Value type; never infer it from message text or
	// the expected native row. Keep this test-only adapter out of product code.
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		value := reflect.ValueOf(cause)
		if value.Kind() != reflect.Pointer || value.IsNil() {
			continue
		}
		payload := value.Elem()
		if payload.Kind() != reflect.Struct || payload.Type().PkgPath() != reflect.TypeOf(vm.Value{}).PkgPath() || payload.Type().Name() != "apexThrowError" {
			continue
		}
		thrown := payload.FieldByName("value")
		if thrown.IsValid() && thrown.Type() == reflect.TypeOf(vm.Value{}) {
			return thrown.FieldByName("Type").String(), cause.Error()
		}
	}
	return "", err.Error()
}

func asyncStackDepthTypeMatches(outcome, got, want string) bool {
	return got == want && (outcome == "observed" || got != "")
}

func TestAsyncStackDepthExceptionTypeComparisonRoutes(t *testing.T) {
	for _, route := range []string{"anonymous", "istest"} {
		outcome := "thrown"
		if route == "anonymous" {
			outcome = "job-error"
		}
		for _, got := range []string{"", "System.DmlException", "System.AsyncException"} {
			t.Run(route+"/"+got, func(t *testing.T) {
				want := got == "System.AsyncException"
				if matched := asyncStackDepthTypeMatches(outcome, got, "System.AsyncException"); matched != want {
					t.Fatalf("type %q matched native AsyncException: %t, want %t", got, matched, want)
				}
			})
		}
	}
}

// Inject only the exception in the owned native project's uncaught child route.
// This is a harness mutation check, not a new Salesforce behavior expectation.
// Expectations: async_stack_depth/api{62,67}/org.tsv:19,45 and ORG-RUN.txt.
func TestAsyncStackDepthExceptionTypeProjectRoutes(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "async_stack_depth")
	for _, api := range []string{"62.0", "67.0"} {
		for _, injected := range []string{"System.AsyncException", "System.DmlException"} {
			t.Run(api+"/"+injected, func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join(dir, "project")
				err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					source, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					text := strings.ReplaceAll(string(source), "62.0", api)
					if entry.Name() == "StackDepthProbe.cls" {
						start := strings.Index(text, "        public void child() {")
						end := strings.Index(text, "        public void execute(QueueableContext context)")
						if start < 0 || end <= start {
							t.Fatal("missing owned child route")
						}
						text = text[:start] + "        public void child() { throw new " + injected + "('Maximum stack depth has been reached.'); }\n" + text[end:]
					}
					name, err := filepath.Rel(project, path)
					if err != nil {
						return err
					}
					writeFile(t, filepath.Join(root, name), text)
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("mutated project compile: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, route := range []string{"anonymous", "istest"} {
					t.Run(route, func(t *testing.T) {
						machine := runner.newMachine()
						source, err := os.ReadFile(filepath.Join(dir, "anonymous", "overflow_flush.apex"))
						if err != nil {
							t.Fatal(err)
						}
						program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						outcome := "job-error"
						if route == "istest" {
							selected := Discover(index, Options{SelectedClasses: []string{"StackDepthProbeTest"}, SelectedMethod: "overflow_flush"})
							if len(selected) != 1 {
								t.Fatalf("named selection: %v", selected)
							}
							methods, methodErrors := compileTestMethodsForIndex(&index, selected)
							programs, programErrors := compileTestInvokePrograms(selected)
							key := testCaseKey(selected[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
							}
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, err = machine.ExecuteInClass(programs[key], selected[0].ClassName)
							outcome = "thrown"
						} else {
							var result vm.Result
							result, err = machine.Execute(program)
							if err != nil {
								t.Fatalf("submission: %v", err)
							}
							err = machine.DrainAsync(&result)
						}
						if err == nil {
							t.Fatal("missing injected exception")
						}
						for _, fault := range []error{err, fmt.Errorf("dispatch context: %w", err)} {
							gotType, message := asyncStackDepthFaultDetails(fault)
							if message != "Maximum stack depth has been reached." {
								t.Fatalf("message: %q", message)
							}
							matched := asyncStackDepthTypeMatches(outcome, gotType, "System.AsyncException")
							if want := injected == "System.AsyncException"; matched != want {
								t.Fatalf("%T type=%q: native match=%t, want %t for injected %s", err, gotType, matched, want, injected)
							}
							if gotType != injected {
								t.Errorf("%T type=%q, want actual injected %s", err, gotType, injected)
							}
						}
					})
				}
			})
		}
	}
}

// Project-runner twin of the istest rows: the real Run route, API 62.0 and 67.0.
// Rows: async_stack_depth/api{62,67}/org.tsv (route == istest).
func TestAsyncStackDepthProjectRunnerNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "async_stack_depth")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(source), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			file, err := os.Open(filepath.Join(dir, "api"+api[:2], "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			reader := csv.NewReader(file)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, row := range rows[1:] {
				if row[1] != "istest" {
					continue
				}
				count++
				t.Run(row[2], func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"StackDepthProbeTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("missing project-runner route: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil {
						t.Fatal("project runner produced neither observation nor exception")
					}
					outcome, exceptionType, message, values := "observed", "", "", []string{}
					if payload, ok := strings.CutPrefix(problem.Message, "Assertion Failed: STACK|"); ok && strings.HasSuffix(problem.Type, "AssertException") {
						if err := json.Unmarshal([]byte(payload), &values); err != nil {
							t.Fatal(err)
						}
					} else {
						outcome, exceptionType, message = "thrown", problem.Type, problem.Message
					}
					var wantValues []string
					if err := json.Unmarshal([]byte(row[6]), &wantValues); err != nil {
						t.Fatal(err)
					}
					if outcome != row[3] || !asyncStackDepthTypeMatches(outcome, exceptionType, row[4]) || message != row[5] || !reflect.DeepEqual(values, wantValues) {
						t.Fatalf("native=(%s,%q,%q,%s); Glade=(%s,%q,%q,%v)", row[3], row[4], row[5], row[6], outcome, exceptionType, message, values)
					}
				})
			}
			if count != 22 {
				t.Fatalf("project-runner rows: %d, want 22", count)
			}
		})
	}
}
