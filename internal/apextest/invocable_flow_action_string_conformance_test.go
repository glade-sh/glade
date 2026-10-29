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

func TestInvocableFlowActionStringNativeConformance(t *testing.T) {
	var data struct {
		APIVersions []string `json:"apiVersions"`
		Routes      []string `json:"routes"`
		Rows        []struct {
			ID       string                    `json:"id"`
			Code     string                    `json:"code"`
			Expected map[string]map[string]any `json:"expected"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/invocable_flow_action_string/answers.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,istest" || len(data.Rows) != 2 {
		t.Fatal("incomplete native matrix")
	}
	for _, row := range data.Rows {
		if row.ID == "" || len(row.Expected) != 6 {
			t.Fatalf("incomplete row %q", row.ID)
		}
	}

	flow, err := os.ReadFile("testdata/conformance/invocable_flow_action_string/ProbeTextEcho.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			flowPath := filepath.Join(root, "force-app/main/default/flows/ProbeTextEcho.flow-meta.xml")
			writeFile(t, flowPath, strings.ReplaceAll(string(flow), "{{API}}", api))
			baseIndex := loadTestIndex(t, root)
			if baseIndex.HasErrors() {
				t.Fatal(baseIndex.Diagnostics)
			}
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})

			for _, row := range data.Rows {
				for _, route := range data.Routes {
					t.Run(route+"/"+row.ID, func(t *testing.T) {
						want, ok := row.Expected[api+"/"+route]
						if !ok {
							t.Fatal("missing captured cell")
						}
						body := row.Code
						var execErr error
						if route == "anonymous" {
							analysis := sema.AnalyzeAnonymous(baseIndex, body, api)
							if analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							_, execErr = base.execute(program)
						} else {
							className := "TextFlowActionObserved" + strings.ToUpper(row.ID[:1]) + row.ID[1:]
							path := filepath.Join(root, "force-app/main/default/classes", className+".cls")
							writeFile(t, path, "@IsTest private class "+className+" { @IsTest static void observe(){"+body+"} }")
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							index := loadTestIndex(t, root)
							if index.HasErrors() || sema.Analyze(index).HasErrors() {
								t.Fatalf("named source failed compilation: %v %v", index.Diagnostics, sema.Analyze(index).Diagnostics)
							}
							cases := Discover(index, Options{SelectedClasses: []string{className}, SelectedMethod: "observe"})
							if len(cases) != 1 {
								t.Fatal("missing named route")
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
							}
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
							machine := runner.newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, execErr = machine.ExecuteInClass(programs[key], className)
						}
						assertInvocableFlowStringCell(t, row.ID, want, execErr)
					})
				}
			}
		})
	}
}

func assertInvocableFlowStringCell(t *testing.T, id string, want map[string]any, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("missing observation marker for %s: %v", id, execErr)
	}
	_, payload, ok := strings.Cut(runtimeErr.ExceptionMessage(), "FLOW_CELL|"+id+"|")
	if !ok {
		t.Fatalf("missing observation marker for %s: %v", id, execErr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native %v; Glade %v", want, got)
	}
}
