package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowLoopRelationshipNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_loop_relationship"
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, name := range []string{"RouteProbe", "RouteProbeTest"} {
				p := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, p, read("sources/classes/"+name+".cls"))
				writeFile(t, p+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/flows/RouteLoop.flow-meta.xml"), read("metadata/"+version+"/flows/RouteLoop.flow-meta.xml"))
			idx := loadTestIndex(t, root)
			for _, route := range []string{"anonymous", "istest", "istest_warm"} {
				t.Run(route, func(t *testing.T) {
					nativeRoute := route
					if route == "istest_warm" {
						nativeRoute = "istest"
					}
					rows := strings.Split(strings.TrimSpace(read("native/"+version+"/"+nativeRoute+"/org.tsv")), "\n")
					if len(rows) != 2 || rows[0] != "id\tvalue" {
						t.Fatal("invalid native rows")
					}
					parts := strings.SplitN(rows[1], "\t", 2)
					if len(parts) != 2 || parts[0] != "loop" {
						t.Fatal("invalid native row")
					}
					var want map[string]any
					if err := json.Unmarshal([]byte(parts[1]), &want); err != nil {
						t.Fatal(err)
					}
					var message string
					if route != "anonymous" {
						options := Options{SelectedClasses: []string{"RouteProbeTest"}, SelectedMethod: "observe", NoDiskCache: route == "istest", Parallelism: 1}
						if route == "istest_warm" {
							first := Run(idx, options)
							if len(first.Suites) != 1 || len(first.Suites[0].Cases) != 1 || first.Suites[0].Cases[0].Problem == nil {
								t.Fatalf("missing warmup probe: %s", firstRunProblem(first))
							}
						}
						run := Run(idx, options)
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
							t.Fatalf("missing probe: %s", firstRunProblem(run))
						}
						message = run.Suites[0].Cases[0].Problem.Message
					} else {
						org := orgFromIndex(idx)
						org.APIVersion = api
						runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
						program, err := vm.CompileAnonymousWithOptions("System.assert(false, 'PROBE|' + JSON.serialize(RouteProbe.observe()));", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = runner.newMachine().Execute(program)
						if err == nil {
							t.Fatal("missing observation assertion")
						}
						message = err.Error()
					}
					_, payload, ok := strings.Cut(message, "PROBE|")
					if !ok {
						t.Fatalf("missing observation: %s", message)
					}
					var got map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatalf("observation %q: %v", payload, err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("native %v; Glade %v", want, got)
					}
				})
			}
		})
	}
}
