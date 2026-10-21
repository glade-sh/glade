package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowParentTextAssignmentNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_parent_text_assignment"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	classes := map[string]string{
		"MeadowParentTextProbe":     read("sources/classes/MeadowParentTextProbe.cls"),
		"MeadowParentTextProbeTest": read("sources/classes/MeadowParentTextProbeTest.cls"),
	}
	anonymous := read("sources/anonymous.apex")
	seedSource := read("sources/anonymous-seed.apex")
	ids := []string{"P01", "P02", "P03", "P04", "P05"}
	var anonymousSeed storage.OrgState
	seeded := false
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FlowParentTextAssignment", api)
			version := "api" + strings.TrimSuffix(api, ".0")
			sources := make(map[string]string)
			for name, source := range classes {
				sources["classes/"+name+".cls"] = source
				path := "classes/" + name + ".cls-meta.xml"
				sources[path] = read("metadata/" + version + "/" + path)
			}
			flowPath := "flows/Meadow_ParentText.flow-meta.xml"
			sources[flowPath] = read("metadata/" + version + "/" + flowPath)
			for _, path := range []string{"objects/Account/fields/MeadowText__c.field-meta.xml", "objects/Contact/fields/MeadowParent__c.field-meta.xml"} {
				sources[path] = read("sources/" + path)
			}
			files := make(map[string]string)
			for path, source := range sources {
				files["force-app/main/default/"+path] = source
			}
			index := flowRefsIndex(t, api, classes, files)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			if len(org.Objects["Contact"].Definition.FlowRules) != 1 {
				t.Fatalf("expected one Contact-triggered flow, got %d", len(org.Objects["Contact"].Definition.FlowRules))
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if api == "62.0" {
				// Anonymous seeding ran once at API62 in a separate transaction.
				analysis := sema.AnalyzeAnonymous(index, seedSource, api)
				program, err := vm.CompileAnonymousWithOptions(seedSource, vm.CompileOptions{APIVersion: api})
				if err != nil || analysis.HasErrors() {
					t.Fatalf("native anonymous seed accepted: %v %v", err, analysis.Diagnostics)
				}
				machine := base.newMachine()
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("anonymous seed failed: %v", err)
				}
				anonymousSeed, seeded = machine.Org.Clone(), true
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					root := "native/" + version + "/" + route + "/"
					run := read(root + "RUN-FACTS.txt")
					for name, value := range map[string]string{
						"api": api, "context": route, "rows": "5", "native_complete": "true", "deployment_success": "true",
						"helper_class_api": api, "wrapper_class_api": api, "flow_api": api,
					} {
						if !strings.Contains("\n"+run, "\n"+name+"="+value+"\n") {
							t.Fatalf("missing native capture fact: %s", name)
						}
					}
					for path, source := range sources {
						if !strings.Contains(run, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture detached from exact source: %s", path)
						}
					}
					for name, source := range map[string]string{"anonymous": anonymous, "seed": seedSource} {
						if !strings.Contains(run, fmt.Sprintf("%s_source_sha256=%x\n", name, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture detached from %s source", name)
						}
					}
					rows := strings.Split(strings.TrimSuffix(read(root+"org.tsv"), "\n"), "\n")
					if len(rows) != 6 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native TSV")
					}
					want := make(map[string]map[string]any)
					for n, id := range ids {
						row := strings.SplitN(rows[n+1], "\t", 5)
						if len(row) != 5 {
							t.Fatalf("invalid native TSV row: %s", rows[n+1])
						}
						var cell struct {
							Outcome string
							Value   map[string]any
						}
						decoder := json.NewDecoder(strings.NewReader(row[4]))
						decoder.DisallowUnknownFields()
						if row[0] != id || row[1] != "FLOW_PARENT_TEXT_ASSIGNMENT" || row[2] != route || row[3] != "runtime" || decoder.Decode(&cell) != nil || cell.Outcome != "runtime" {
							t.Fatalf("invalid native observation: %v", row)
						}
						if len(cell.Value) != 3 {
							t.Fatalf("incomplete native observation: %v", cell.Value)
						}
						for _, field := range []string{"description", "email", "exception"} {
							value, exists := cell.Value[field]
							if !exists {
								t.Fatalf("native observation missing %s", field)
							}
							if value != nil {
								if _, ok := value.(string); !ok {
									t.Fatalf("invalid native %s: %v", field, value)
								}
							}
						}
						want[id] = cell.Value
					}
					var message string
					if route == "anonymous" {
						if !seeded {
							t.Fatal("missing separately committed API62 seed")
						}
						seed := anonymousSeed.Clone()
						seed.APIVersion = api
						contact := seed.Objects["Contact"]
						contact.Definition.FlowRules = append([]storage.FlowRule(nil), org.Objects["Contact"].Definition.FlowRules...)
						seed.Objects["Contact"] = contact
						machine := base.newMachine()
						machine.SetOrg(&seed)
						analysis := sema.AnalyzeAnonymous(index, anonymous, api)
						program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("native anonymous accepted: %v %v", err, analysis.Diagnostics)
						}
						_, execErr := machine.Execute(program)
						var runtimeErr *vm.RuntimeError
						if !errors.As(execErr, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
							t.Fatalf("missing anonymous assertion transport: %v", execErr)
						}
						message = runtimeErr.ExceptionMessage()
					} else {
						// Run establishes the actual @TestSetup transaction boundary.
						run := Run(index, Options{SelectedClasses: []string{"MeadowParentTextProbeTest"}, SelectedMethod: "observeParentText", NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("named observation selection: %#v", run.Summary())
						}
						observed := run.Suites[0].Cases[0]
						if observed.Problem == nil || flowRefsExceptionType(observed.Problem.Type) != "System.AssertException" {
							t.Fatalf("missing named assertion transport: %#v", observed)
						}
						message = observed.Problem.Message
					}
					if strings.Count(message, "PROBE|") != 1 {
						t.Fatalf("missing captured observation envelope: %s", message)
					}
					_, payload, _ := strings.Cut(message, "PROBE|")
					var got map[string]map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != 5 {
						t.Fatalf("invalid observation: %s (%v)", payload, err)
					}
					countRoute := route
					if route == "istest" {
						countRoute = "isTest"
					}
					for _, id := range ids {
						counts.Run(t, countRoute, id, id, func(t *testing.T) {
							if !reflect.DeepEqual(got[id], want[id]) {
								t.Fatalf("native %#v; Glade %#v", want[id], got[id])
							}
						})
					}
				})
			}
		})
	}
}
