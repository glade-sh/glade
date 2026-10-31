package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The omitted RecordTypeId route is distinct from the explicit master ID
// rejection in the original FlowRefs fixture. See flow_refs_omitted_recordtype/org.tsv.
func TestFlowRefsOmittedRecordTypeNative(t *testing.T) {
	dir := "testdata/conformance/flow_refs_omitted_recordtype"
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	object, flow, anonymous, named := read("object.xml"), read("flow.xml"), read("anonymous.apex"), read("named.cls")
	want := json.RawMessage(`{"snapshots":[{"picklist":null,"formula":null,"field":null,"result":"Default","phase":"insert"},{"picklist":"Alpha","formula":"Alpha","field":null,"result":"Default","phase":"update"}]}`)
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					files := map[string]string{"force-app/main/default/objects/FProbeFr__c.object": object, "force-app/main/default/flows/FProbeC182FR015_62.flow-meta.xml": strings.Replace(flow, "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>", 1)}
					classes := map[string]string{}
					if route == "istest" {
						classes["FlowBoundary62"] = named
					}
					index := flowRefsIndex(t, api, classes, files)
					p, err := project.Load(index.Project.Root)
					if err != nil {
						t.Fatal(err)
					}
					automations, err := automation.LoadProject(p)
					if err != nil || len(automations.Diagnostics) != 0 {
						t.Fatalf("Flow load: %v %v", err, automations.Diagnostics)
					}
					analysis := sema.Analyze(index)
					if index.HasErrors() || analysis.HasErrors() {
						t.Fatalf("analysis: %v %v", index.Diagnostics, analysis.Diagnostics)
					}
					baseIndex := flowRefsIndex(t, api, nil, nil)
					base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{})
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
					var execErr error
					if route == "anonymous" {
						a := sema.AnalyzeAnonymous(index, anonymous, api)
						program, e := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if e != nil || a.HasErrors() {
							t.Fatalf("anonymous compile: %v %v", e, a.Diagnostics)
						}
						_, execErr = runner.execute(program)
					} else {
						cases := Discover(index, Options{SelectedClasses: []string{"FlowBoundary62"}, SelectedMethod: "observe"})
						if len(cases) != 1 {
							t.Fatalf("cases: %d", len(cases))
						}
						methods, errs := compileTestMethodsForIndex(&index, cases)
						programs, perrs := compileTestInvokePrograms(cases)
						key := testCaseKey(cases[0])
						if errs[key] != nil || perrs[key] != nil {
							t.Fatalf("compile: %v %v", errs[key], perrs[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
					}
					flowRefsPayload(t, "FR15_NULL", want, execErr)
				})
			}
		})
	}
}

// A scalar DeveloperName cannot be traversed as a relationship. Salesforce
// rejects this metadata at deployment; the Apex execution routes are unreachable.
func TestFlowRefsOmittedRecordTypeInvalidPathNative(t *testing.T) {
	dir := "testdata/conformance/flow_refs_omitted_recordtype"
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	object, flow := read("object.xml"), read("rejection.xml")
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			files := map[string]string{"force-app/main/default/objects/FProbeFr__c.object": object, "force-app/main/default/flows/FProbeReject.flow-meta.xml": strings.Replace(flow, "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>", 1)}
			index := flowRefsIndex(t, api, nil, files)
			p, err := project.Load(index.Project.Root)
			if err != nil {
				t.Fatal(err)
			}
			automations, err := automation.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range automations.Diagnostics {
				if d.Severity == diagnostic.Error && strings.HasSuffix(d.File, "FProbeReject.flow-meta.xml") {
					return
				}
			}
			t.Fatalf("native rejected invalid Flow path; Glade diagnostics: %v", automations.Diagnostics)
		})
	}
}
