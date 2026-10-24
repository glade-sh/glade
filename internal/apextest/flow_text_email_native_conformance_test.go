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
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowTextEmailNativeConformance(t *testing.T) {
	runFlowTextEmailConformance(t, false)
}

// Reuse captured text observations through the other Apex entry point.
// These are local route twins; native rows were captured through Flow.Interview.
// Omitted String inputs exercise null defaults because explicit null scalar
// inputs and email actions retain their Invocable.Action adapter boundaries.
func TestFlowTextEmailInvocableRouteTwins(t *testing.T) {
	runFlowTextEmailConformance(t, true)
}

func runFlowTextEmailConformance(t *testing.T, invocable bool) {
	const fixture = "../dml/testdata/conformance/flow_text_email"
	read := func(t *testing.T, path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	parse := func(t *testing.T, path string) [][]string {
		t.Helper()
		var rows [][]string
		for _, line := range strings.Split(strings.TrimSuffix(read(t, path), "\n"), "\n") {
			row := strings.Split(line, "\t")
			if len(rows) > 0 && len(row) != len(rows[0]) {
				t.Fatalf("invalid TSV row in %s: %s", path, line)
			}
			rows = append(rows, row)
		}
		return rows
	}
	cases := parse(t, "cases.tsv")
	if len(cases) != 33 || strings.Join(cases[0], "\t") != "id\tfamily\troute\toperator\tinput\tcomparison\tlogEmailOnSend\tresource" {
		t.Fatal("incomplete Flow text/email case table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("api"+api, func(t *testing.T) {
			directory := "api" + strings.TrimSuffix(api, ".0")
			capture := read(t, filepath.Join(directory, "RUN-FACTS.txt"))
			for _, fact := range []string{"api=" + api, "native_complete=true", "deployment_success=true", "anonymous_rows=32", "istest_rows=32"} {
				if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
					t.Fatalf("missing native capture fact: %s", fact)
				}
			}
			observations := parse(t, filepath.Join(directory, "org.tsv"))
			if len(observations) != 65 || strings.Join(observations[0], "\t") != "context\tid\tobservation" {
				t.Fatal("incomplete native context matrix")
			}
			want := map[string]any{}
			for _, row := range observations[1:] {
				key := row[0] + "/" + row[1]
				if _, exists := want[key]; exists {
					t.Fatalf("duplicate native row: %s", key)
				}
				var value any
				if err := json.Unmarshal([]byte(row[2]), &value); err != nil {
					t.Fatal(err)
				}
				want[key] = value
			}
			root := t.TempDir()
			sourceRoot := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				body = []byte(strings.ReplaceAll(string(body), "62.0", api))
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256(body))) {
					t.Fatalf("native source changed: %s", rel)
				}
				if invocable && filepath.Base(path) == "FlowTextEmailProbe.cls" {
					create := regexp.MustCompile(`Flow.Interview interview=Flow.Interview.createInterview\(([^,]+),\s*(new Map<String,Object>\{[^\n]*?\})\);`)
					text := create.ReplaceAllString(string(body), `Invocable.Action action=Invocable.Action.createCustomAction('flow',$1); action.setInvocations(new List<Map<String,Object>>{$2});`)
					if strings.Count(text, "action.setInvocations(") != 26 {
						t.Fatal("incomplete Invocable.Action route substitution")
					}
					text = strings.ReplaceAll(text, "interview.start();", "Invocable.Action.Result flowResult=action.invoke()[0]; System.assertEquals(true,flowResult.isSuccess(),JSON.serialize(flowResult.getErrors()));")
					text = strings.ReplaceAll(text, "interview.getVariableValue(", "flowResult.getOutputParameters().get(")
					text = strings.ReplaceAll(text, "'InputText'=>null", "")
					body = []byte(text)
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			testCases := Discover(index, Options{SelectedClasses: []string{"FlowTextEmailProbeTest"}})
			if len(testCases) != 32 {
				t.Fatalf("named test routes = %d, want 32", len(testCases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, testCases)
			programs, programErrors := compileTestInvokePrograms(testCases)
			testByMethod := make(map[string]TestCase)
			for _, testCase := range testCases {
				testByMethod[testCase.MethodName] = testCase
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, context := range []string{"anonymous", "istest"} {
				t.Run(context, func(t *testing.T) {
					for _, row := range cases[1:] {
						id := row[0]
						if invocable && row[1] != "text" {
							continue
						}
						t.Run(id, func(t *testing.T) {
							key := context + "/" + id
							expected, ok := want[key]
							if !ok {
								t.Fatalf("missing native row: %s", key)
							}
							source := "System.debug(LoggingLevel.ERROR,'OBS|'+JSON.serialize(FlowTextEmailProbe." + id + "()));"
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							machine := runner.newMachine()
							var result vm.Result
							if context == "istest" {
								testCase, exists := testByMethod[id]
								if !exists {
									t.Fatalf("missing named test route: %s", id)
								}
								testKey := testCaseKey(testCase)
								if methodErrors[testKey] != nil || programErrors[testKey] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[testKey], programErrors[testKey])
								}
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[testKey]); err != nil {
									t.Fatal(err)
								}
								result, err = machine.ExecuteTestInClass(programs[testKey], testCase.ClassName)
							} else {
								result, err = machine.Execute(program)
							}
							payload := ""
							if context == "istest" {
								var runtimeErr *vm.RuntimeError
								if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
									t.Fatalf("native=%v; Glade runtime before observation: %v", expected, err)
								}
								_, payload, _ = strings.Cut(runtimeErr.ExceptionMessage(), "OBS|")
							} else {
								if err != nil {
									t.Fatalf("native=%v; Glade runtime: %v", expected, err)
								}
								for _, line := range result.Debug {
									if value, found := strings.CutPrefix(line, "OBS|"); found {
										payload = value
									}
								}
							}
							if payload == "" {
								t.Fatal("Flow observation was not emitted")
							}
							var actual any
							if err := json.Unmarshal([]byte(payload), &actual); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(actual, expected) {
								t.Fatalf("native=%v; Glade=%v", expected, actual)
							}
						})
					}
				})
			}
		})
	}
}
