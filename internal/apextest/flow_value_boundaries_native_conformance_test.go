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

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const flowValueBoundaryFixture = "../dml/testdata/conformance/flow_value_boundaries"

func flowValueBoundaryRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(flowValueBoundaryFixture, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func flowValueBoundaryProject(t *testing.T, api string) (string, map[string]any, [][]string) {
	t.Helper()
	directory := "api" + strings.TrimSuffix(api, ".0")
	capture := flowValueBoundaryRead(t, directory+"/RUN-FACTS.txt")
	for _, fact := range []string{"api=" + api, "native_complete=true", "deployment_success=true"} {
		if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
			t.Fatalf("missing native fact: %s", fact)
		}
	}
	var cases [][]string
	for _, line := range strings.Split(strings.TrimSpace(flowValueBoundaryRead(t, "cases.tsv")), "\n")[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 4 {
			t.Fatal("invalid case table")
		}
		cases = append(cases, cells)
	}
	want := map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(flowValueBoundaryRead(t, directory+"/org.tsv")), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 {
			t.Fatal("invalid native row")
		}
		key := cells[0] + "/" + cells[1]
		if _, ok := want[key]; ok {
			t.Fatal("duplicate native row")
		}
		var v any
		if err := json.Unmarshal([]byte(cells[2]), &v); err != nil {
			t.Fatal(err)
		}
		want[key] = v
	}
	if len(cases) != 124 || len(want) != 2*len(cases) {
		t.Fatalf("incomplete native matrix: cases=%d rows=%d", len(cases), len(want))
	}
	root := t.TempDir()
	sourceRoot := filepath.Join(flowValueBoundaryFixture, "project")
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
		writeFile(t, filepath.Join(root, rel), string(body))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return root, want, cases
}

func flowValueBoundaryCompare(t *testing.T, message string, want any) {
	t.Helper()
	_, payload, found := strings.Cut(message, "OBS|")
	var got any
	if !found || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("missing native observation: %s", message)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%v; Glade=%v", want, got)
	}
}

func TestFlowValueBoundariesNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root, want, rows := flowValueBoundaryProject(t, api)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"FlowValueProbeTest"}})
			if len(cases) != len(rows) {
				t.Fatalf("test count=%d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			byName := map[string]TestCase{}
			for _, c := range cases {
				byName[c.MethodName] = c
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			captureProject, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if err := applyConfiguredTestPermissions(&org, captureProject); err != nil {
				t.Fatal(err)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, context := range []string{"anonymous", "istest"} {
				t.Run(context, func(t *testing.T) {
					for _, row := range rows {
						t.Run(row[0], func(t *testing.T) {
							machine := base.newMachine()
							var runErr error
							if context == "anonymous" {
								source := fmt.Sprintf("System.assert(false,'OBS|'+JSON.serialize(FlowValueProbe.observe('%s','%s','%s')));", row[1], row[2], row[3])
								analysis := sema.AnalyzeAnonymous(index, source, api)
								program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
								if err != nil || analysis.HasErrors() {
									t.Fatalf("compile: %v %v", err, analysis.Diagnostics)
								}
								_, runErr = machine.Execute(program)
							} else {
								c := byName[row[0]]
								key := testCaseKey(c)
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, runErr = machine.ExecuteTestInClass(programs[key], c.ClassName)
							}
							var runtimeErr *vm.RuntimeError
							if !errors.As(runErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
								t.Fatalf("native observation completed; Glade: %v", runErr)
							}
							flowValueBoundaryCompare(t, runtimeErr.ExceptionMessage(), want[context+"/"+row[0]])
						})
					}
				})
			}
		})
	}
}

func TestFlowValueBoundariesTestRunnerNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root, want, rows := flowValueBoundaryProject(t, api)
			index := loadTestIndex(t, root)
			report := Run(index, Options{SelectedClasses: []string{"FlowValueProbeTest"}, RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					count++
					t.Run(c.MethodName, func(t *testing.T) {
						if c.Problem == nil {
							t.Fatalf("missing observation: %+v", c)
						}
						flowValueBoundaryCompare(t, c.Problem.Message, want["istest/"+c.MethodName])
					})
				}
			}
			if count != len(rows) {
				t.Fatalf("runner cases=%d, want %d", count, len(rows))
			}
		})
	}
}

func TestFlowValueBoundariesNativeRejections(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root, _, _ := flowValueBoundaryProject(t, api)
			directory := "api" + strings.TrimSuffix(api, ".0")
			capture := flowValueBoundaryRead(t, directory+"/RUN-FACTS.txt")
			rows := strings.Split(strings.TrimSpace(flowValueBoundaryRead(t, directory+"/rejections.tsv")), "\n")[1:]
			if len(rows) != 9 {
				t.Fatalf("native rejections=%d, want 9", len(rows))
			}
			for _, row := range rows {
				cells := strings.SplitN(row, "\t", 2)
				if len(cells) != 2 {
					t.Fatal("invalid rejection row")
				}
				name := cells[0]
				t.Run(name, func(t *testing.T) {
					source := strings.ReplaceAll(flowValueBoundaryRead(t, "rejected/"+name+".flow-meta.xml"), "62.0", api)
					if !strings.Contains(capture, fmt.Sprintf("rejected_sha256:%s.flow-meta.xml=%x\n", name, sha256.Sum256([]byte(source)))) {
						t.Fatal("native rejected source changed")
					}
					path := filepath.Join(root, "force-app/main/default/flows", name+".flow-meta.xml")
					writeFile(t, path, source)
					defer os.Remove(path)
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					index, err := automation.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					for _, flow := range index.Flows {
						if flow.File == path && len(flow.Rules) != 0 {
							t.Fatal("native rejects metadata; Glade retained executable rules")
						}
					}
					for _, d := range index.Diagnostics {
						if d.File == path && d.Severity == diagnostic.Error {
							return
						}
					}
					t.Fatalf("native rejects metadata: %s; Glade accepted", cells[1])
				})
			}
		})
	}
}
