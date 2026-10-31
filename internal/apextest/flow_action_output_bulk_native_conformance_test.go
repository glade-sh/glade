package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowActionOutputBulkNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/flow_action_output_bulk"
	flow, err := os.ReadFile(filepath.Join(dir, "OwnedOutputProbe.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/flows/OwnedOutputProbe.flow-meta.xml"), strings.ReplaceAll(string(flow), "{{API}}", api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					var want []any
					for _, line := range strings.Split(string(rows), "\n") {
						fields := strings.SplitN(line, "\t", 3)
						if len(fields) == 3 && fields[0] == api && fields[1] == route {
							if err := json.Unmarshal([]byte(fields[2]), &want); err != nil {
								t.Fatal(err)
							}
						}
					}
					if len(want) != 2 {
						t.Fatal("missing native observation")
					}
					var execErr error
					if route == "anonymous" {
						analysis := sema.AnalyzeAnonymous(index, string(body), api)
						if analysis.HasErrors() {
							t.Fatal(analysis.Diagnostics)
						}
						p, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, execErr = base.execute(p)
					} else {
						name := "OwnedOutputProbeTest"
						path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
						writeFile(t, path, "@IsTest private class "+name+" { @IsTest static void observe(){"+string(body)+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						idx := loadTestIndex(t, root)
						if idx.HasErrors() || sema.Analyze(idx).HasErrors() {
							t.Fatalf("named compile: %v %v", idx.Diagnostics, sema.Analyze(idx).Diagnostics)
						}
						cases := Discover(idx, Options{SelectedClasses: []string{name}, SelectedMethod: "observe"})
						if len(cases) != 1 {
							t.Fatal("missing test")
						}
						methods, me := compileTestMethods(cases)
						programs, pe := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if me[key] != nil || pe[key] != nil {
							t.Fatalf("compile: %v %v", me[key], pe[key])
						}
						runner := newConformanceRunner(t, idx, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						_, execErr = machine.ExecuteInClass(programs[key], name)
					}
					var e *vm.RuntimeError
					if !errors.As(execErr, &e) || e.Type != "System.AssertException" {
						t.Fatalf("missing observation: %v", execErr)
					}
					_, payload, ok := strings.Cut(e.ExceptionMessage(), "ROAD195_RESULT|")
					if !ok {
						t.Fatalf("missing marker: %v", execErr)
					}
					var got []any
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					for _, item := range got {
						if m, ok := item.(map[string]any); ok {
							if outputs, ok := m["outputs"].(map[string]any); ok {
								delete(outputs, "Flow__InterviewGuid")
							}
						}
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native %v; Glade %v", want, got)
					}
				})
			}
		})
	}
}

// Project and warm-cache twins use the same captured operation and assert its
// stable output values through the normal named test runner.
func TestFlowActionOutputBulkProjectRoutes(t *testing.T) {
	const dir = "testdata/conformance/flow_action_output_bulk"
	flow, err := os.ReadFile(filepath.Join(dir, "OwnedOutputProbe.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	operation := strings.Replace(string(body), "System.assert(false,'ROAD195_RESULT|'+JSON.serialize(out));", `System.assertEquals(2,rs.size());
for (Integer i=0;i<2;i++) { System.assert(rs[i].isSuccess()); Map<String,Object> values=rs[i].getOutputParameters(); System.assertEquals(null,values.get('NullText')); System.assertEquals('',values.get('EmptyText')); System.assertEquals(i==0?'alpha':'beta',values.get('Echo')); System.assertEquals('Finished',values.get('Flow__InterviewStatus')); System.assertEquals(new List<String>{'first','second'},values.get('Items')); }`, 1)
	if operation == string(body) {
		t.Fatal("missing capture marker")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/flows/OwnedOutputProbe.flow-meta.xml"), strings.ReplaceAll(string(flow), "{{API}}", api))
			path := filepath.Join(root, "force-app/main/default/classes/OwnedOutputProbeTest.cls")
			writeFile(t, path, "@IsTest private class OwnedOutputProbeTest { @IsTest static void observe(){"+operation+"} }")
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			for _, runName := range []string{"cold", "warm"} {
				t.Run(runName, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"OwnedOutputProbeTest"}, Parallelism: 1})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("summary=%+v problem=%s", s, firstRunProblem(run))
					}
				})
			}
		})
	}
}
