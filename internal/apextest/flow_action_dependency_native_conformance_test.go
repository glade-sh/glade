package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce rejects the Flow at deployment without the action class; Glade
// loads metadata offline, so the missing-source control is checked at execution.
func TestFlowActionDependencyNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_action_dependency"
	read := func(name string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	rows := read("org.tsv")
	for _, api := range []string{"62.0", "67.0"} {
		for _, loaded := range []bool{true, false} {
			name := "unloaded"
			if loaded {
				name = "loaded"
			}
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				base := filepath.Join(root, "force-app/main/default")
				flow := strings.Replace(read("DependencyProbe.flow-meta.xml"), "<apiVersion>67.0", "<apiVersion>"+api, 1)
				writeFile(t, filepath.Join(base, "flows/DependencyProbe.flow-meta.xml"), flow)
				testSource := read("DependencyProbeTest.cls")
				if loaded {
					writeFile(t, filepath.Join(base, "classes/DependencyAction.cls"), read("DependencyAction.cls"))
					writeFile(t, filepath.Join(base, "classes/DependencyAction.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
				} else {
					testSource = `@IsTest private class DependencyProbeTest { @IsTest static void loaded() { Flow.Interview.createInterview('DependencyProbe', new Map<String,Object>()).start(); } }`
				}
				writeFile(t, filepath.Join(base, "classes/DependencyProbeTest.cls"), testSource)
				writeFile(t, filepath.Join(base, "classes/DependencyProbeTest.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
				index := loadTestIndex(t, root)
				run := Run(index, Options{SelectedClasses: []string{"DependencyProbeTest"}, SelectedMethod: "loaded", NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != 1 {
					t.Fatalf("selection: %+v", run.Summary())
				}
				if loaded {
					if !strings.Contains(rows, api+"\tboundary\tistest\tpass: calls=1") || run.Summary().Passed != 1 {
						t.Fatalf("native test route: %s", firstRunProblem(run))
					}
				} else if run.Summary().Passed != 0 || !strings.Contains(firstRunProblem(run), "no static @InvocableMethod found") {
					t.Fatalf("unloaded action did not report missing source: %s", firstRunProblem(run))
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				source := read("anonymous.apex")
				if !loaded {
					source = `Flow.Interview.createInterview('DependencyProbe', new Map<String,Object>()).start();`
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.newMachine().Execute(program)
				if loaded {
					if !strings.Contains(rows, api+"\tboundary\tanonymous\tpass: calls=1") || err != nil {
						t.Fatalf("native anonymous route: %v", err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "no static @InvocableMethod found") {
					t.Fatalf("unloaded anonymous action: %v", err)
				}
			})
		}
	}
}
