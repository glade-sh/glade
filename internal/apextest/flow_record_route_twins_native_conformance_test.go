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
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const flowRecordRouteFixture = "testdata/conformance/flow_record_route_twins"

func TestFlowRecordRouteTwinsNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := copyFlowRecordRouteProject(t, api)
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"BoundaryFlowProbeTest"}})
			if len(cases) != 15 {
				t.Fatalf("route count = %d, want 15", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			want := readFlowRecordRouteRows(t, api)
			t.Run("istest", func(t *testing.T) {
				for _, testCase := range cases {
					t.Run(testCase.MethodName, func(t *testing.T) {
						key := testCaseKey(testCase)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := base.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], testCase.ClassName)
						compareFlowRecordRouteObservation(t, err, want["istest/"+testCase.MethodName])
					})
				}
			})
			t.Run("anonymous", func(t *testing.T) {
				source, err := os.ReadFile(filepath.Join(flowRecordRouteFixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				analysis := sema.AnalyzeAnonymous(index, string(source), api)
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil || analysis.HasErrors() {
					t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
				}
				_, err = base.newMachine().Execute(program)
				observations := make(map[string]any)
				for name, value := range want {
					if strings.HasPrefix(name, "anonymous/") {
						observations[strings.TrimPrefix(name, "anonymous/")] = value
					}
				}
				if len(observations) != 13 {
					t.Fatalf("anonymous route count = %d", len(observations))
				}
				compareFlowRecordRouteObservation(t, err, observations)
			})
		})
	}
}

func compareFlowRecordRouteObservation(t *testing.T, err error, want any) {
	t.Helper()
	if want == nil {
		t.Fatal("missing native route")
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(err, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
		t.Fatalf("native observation completed; Glade: %v", err)
	}
	_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "OBS|")
	var got any
	if !found || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("invalid observation: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native=%v; Glade=%v", want, got)
	}
}

func readFlowRecordRouteRows(t *testing.T, api string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(flowRecordRouteFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 29 || lines[0] != "context\troute\tobservation" {
		t.Fatal("incomplete native route matrix")
	}
	rows := make(map[string]any)
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 {
			t.Fatal("invalid native row")
		}
		key := cells[0] + "/" + cells[1]
		if _, exists := rows[key]; exists {
			t.Fatalf("duplicate native row: %s", key)
		}
		var value any
		if json.Unmarshal([]byte(cells[2]), &value) != nil {
			t.Fatalf("invalid native observation: %s", key)
		}
		rows[key] = value
	}
	return rows
}

func copyFlowRecordRouteProject(t *testing.T, api string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(flowRecordRouteFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	capture := string(raw)
	if !strings.Contains(capture, "native_complete=true\n") || !strings.Contains(capture, "api="+api+"\n") {
		t.Fatal("incomplete native capture")
	}
	root := t.TempDir()
	sourceRoot := filepath.Join(flowRecordRouteFixture, "project")
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source = []byte(strings.ReplaceAll(string(source), "62.0", api))
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256(source))) {
			t.Fatalf("native source changed: %s", rel)
		}
		writeFile(t, filepath.Join(root, rel), string(source))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(flowRecordRouteFixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capture, fmt.Sprintf("anonymous_sha256=%x\n", sha256.Sum256(source))) {
		t.Fatal("native anonymous source changed")
	}
	return root
}

func TestFlowRecordRouteTwinsNativeRejections(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"BoundaryBeforeSubflow", "BoundaryEventNested"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				root := copyFlowRecordRouteProject(t, api)
				path := filepath.Join(flowRecordRouteFixture, "rejected", name+".flow-meta.xml")
				source, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				source = []byte(strings.ReplaceAll(string(source), "62.0", api))
				capture, err := os.ReadFile(filepath.Join(flowRecordRouteFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "RUN-FACTS.txt"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(capture), fmt.Sprintf("rejected_sha256:%s=%x\n", name, sha256.Sum256(source))) {
					t.Fatal("native rejection source changed")
				}
				rows, err := os.ReadFile(filepath.Join(flowRecordRouteFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "rejections.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(rows), name+"\tmetadata_rejected\tCallChild (Subflow) - A flow can’t include Subflow elements") {
					t.Fatal("missing native metadata rejection")
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/flows", name+".flow-meta.xml"), string(source))
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				index, err := automation.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				for _, d := range index.Diagnostics {
					if d.Severity == diagnostic.Error && strings.HasSuffix(d.File, name+".flow-meta.xml") && strings.Contains(d.Message, "Subflow") {
						return
					}
				}
				t.Fatalf("native rejects Subflow for this trigger type; Glade accepted: %v", index.Diagnostics)
			})
		}
	}
}

func TestFlowRecordRouteTwinsDeliveryNativeConformance(t *testing.T) {
	const fixture = flowRecordRouteFixture + "/delivery"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			capture, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := map[string]any{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				cells := strings.SplitN(line, "\t", 3)
				if len(cells) != 3 {
					t.Fatal("invalid delivery row")
				}
				var observation any
				if err := json.Unmarshal([]byte(cells[2]), &observation); err != nil {
					t.Fatal(err)
				}
				key := cells[0] + "/" + cells[1]
				if rows[key] != nil {
					t.Fatal("duplicate delivery row")
				}
				rows[key] = observation
			}
			if len(rows) != 17 || !strings.Contains(string(capture), "native_complete=true\n") || !strings.Contains(string(capture), "api="+api+"\n") {
				t.Fatal("incomplete delivery capture")
			}
			root := t.TempDir()
			sourceRoot := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				source = []byte(strings.ReplaceAll(string(source), "62.0", api))
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256(source))) {
					t.Fatalf("native delivery source changed: %s", rel)
				}
				writeFile(t, filepath.Join(root, rel), string(source))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"BoundaryDeliveryProbeTest"}})
			if len(cases) != 9 {
				t.Fatalf("delivery routes=%d, want 9", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			t.Run("istest", func(t *testing.T) {
				for _, testCase := range cases {
					t.Run(testCase.MethodName, func(t *testing.T) {
						key := testCaseKey(testCase)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := base.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], testCase.ClassName)
						compareFlowRecordRouteObservation(t, err, rows["istest/"+testCase.MethodName])
					})
				}
			})
			compile := func(t *testing.T, name string) ir.Program {
				t.Helper()
				source, err := os.ReadFile(filepath.Join(fixture, name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(capture), fmt.Sprintf("anonymous_sha256:%s.apex=%x\n", name, sha256.Sum256(source))) {
					t.Fatalf("native anonymous source changed: %s", name)
				}
				analysis := sema.AnalyzeAnonymous(index, string(source), api)
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil || analysis.HasErrors() {
					t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
				}
				return program
			}
			t.Run("anonymous", func(t *testing.T) {
				for _, route := range []string{"direct", "subflow", "direct_static", "subflow_static", "direct_namespace", "subflow_namespace"} {
					t.Run("scheduled_"+route, func(t *testing.T) {
						_, err := base.newMachine().Execute(compile(t, "anonymous-scheduled-"+route))
						compareFlowRecordRouteObservation(t, err, rows["anonymous/scheduled_"+route])
					})
				}
				t.Run("event_committed", func(t *testing.T) {
					machine := base.newMachine()
					if _, err := machine.Execute(compile(t, "anonymous-seed")); err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(compile(t, "anonymous-publish"))
					if err != nil {
						t.Fatal(err)
					}
					var published any
					for _, line := range result.Debug {
						if _, payload, ok := strings.Cut(line, "OBS|"); ok {
							if err := json.Unmarshal([]byte(payload), &published); err != nil {
								t.Fatal(err)
							}
						}
					}
					if !reflect.DeepEqual(published, rows["anonymous/event_publish"]) {
						t.Fatalf("publication: native=%v; Glade=%v", rows["anonymous/event_publish"], published)
					}
					if err := machine.DrainAsync(&result); err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(compile(t, "anonymous-observe"))
					compareFlowRecordRouteObservation(t, err, rows["anonymous/event_committed"])
				})
			})
			t.Run("hosted_schedule_body", func(t *testing.T) { flowScheduledBodyNativeConformance(t, fixture, api, root) })
		})
	}
}

// Replay the body observed under hosted scheduling through Interview.start.
// Credential-free CI does not emulate the wall-clock scheduler.
func flowScheduledBodyNativeConformance(t *testing.T, fixture, api, root string) {
	t.Helper()
	native := filepath.Join(fixture, "native", "api"+strings.TrimSuffix(api, ".0"), "scheduler")
	capture, err := os.ReadFile(filepath.Join(native, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(capture), "native_complete=true\n") || !strings.Contains(string(capture), "api="+api+"\n") {
		t.Fatal("incomplete hosted schedule capture")
	}
	for _, rel := range []string{"force-app/main/default/flows/BoundaryLeaf.flow-meta.xml", "sfdx-project.json"} {
		source, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(capture), fmt.Sprintf("shared_source_sha256:%s=%x\n", rel, sha256.Sum256(source))) {
			t.Fatalf("native scheduler dependency changed: %s", rel)
		}
	}
	read := func(name string) string {
		t.Helper()
		source, err := os.ReadFile(filepath.Join(native, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", name, sha256.Sum256(source))) {
			t.Fatalf("native schedule source changed: %s", name)
		}
		return string(source)
	}
	for _, name := range []string{"BoundaryScheduledDirect", "BoundaryScheduledNested"} {
		writeFile(t, filepath.Join(root, "force-app/main/default/flows", name+".flow-meta.xml"), read(name+".flow-meta.xml"))
	}
	raw, err := os.ReadFile(filepath.Join(native, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 3 || rows[0] != "route\tphone" {
		t.Fatal("incomplete hosted schedule rows")
	}
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	org.APIVersion = api
	base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	for _, line := range rows[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 2 || (cells[0] != "Direct" && cells[0] != "Nested") {
			t.Fatal("invalid schedule row")
		}
		t.Run(cells[0], func(t *testing.T) {
			source := read("seed.apex")
			source += "Account seed=[SELECT Id,Phone FROM Account WHERE Name='Timer " + cells[0] + " " + api + "' LIMIT 1]; Flow.Interview interview=Flow.Interview.createInterview('BoundaryScheduled" + cells[0] + "',new Map<String,Object>{'$Record'=>seed}); interview.start();"
			source += "System.assertEquals('" + cells[1] + "',[SELECT Phone FROM Account WHERE Id=:seed.Id].Phone);"
			analysis := sema.AnalyzeAnonymous(index, source, api)
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil || analysis.HasErrors() {
				t.Fatalf("schedule replay compile: %v %v", err, analysis.Diagnostics)
			}
			if _, err := base.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
