package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowRecordBindingNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_record_binding"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	classes := map[string]string{
		"MeadowFlowBindingProbe":     read("sources/MeadowFlowBindingProbe.cls"),
		"MeadowFlowBindingProbeTest": read("sources/MeadowFlowBindingProbeTest.cls"),
	}
	anonymous := read("sources/anonymous.apex")
	var ids []string
	for n := 1; n <= 25; n++ {
		ids = append(ids, fmt.Sprintf("FRB%02d", n))
	}
	for n := 1; n <= 3; n++ {
		ids = append(ids, fmt.Sprintf("FRT%02d", n))
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FlowRecordBinding", api)
			version := "api" + strings.TrimSuffix(api, ".0")
			sources := make(map[string]string)
			for name, source := range classes {
				sources["classes/"+name+".cls"] = source
				path := "classes/" + name + ".cls-meta.xml"
				sources[path] = read("metadata/" + version + "/" + path)
			}
			files := make(map[string]string)
			for _, name := range []string{"Meadow_RecordBinding", "Meadow_NullRecordTrigger"} {
				path := "flows/" + name + ".flow-meta.xml"
				sources[path] = read("metadata/" + version + "/" + path)
				files["force-app/main/default/"+path] = sources[path]
			}
			index := flowRefsIndex(t, api, classes, files)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"MeadowFlowBindingProbeTest"}, SelectedMethod: "observeRecordBindings"})
			if len(cases) != 1 {
				t.Fatalf("captured named method count = %d", len(cases))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			key := testCaseKey(cases[0])
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			if len(org.Metadata.Flows) != 1 || len(org.Objects["Account"].Definition.FlowRules) != 1 {
				t.Fatalf("expected one autolaunched and one Account-triggered flow, got %d and %d", len(org.Metadata.Flows), len(org.Objects["Account"].Definition.FlowRules))
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					root := "native/" + version + "/" + route + "/"
					run := read(root + "RUN-FACTS.txt")
					for name, value := range map[string]string{
						"api": api, "context": route, "rows": "28", "native_complete": "true",
						"deployment_success": "true", "helper_class_api": api, "wrapper_class_api": api, "flow_api": api,
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
					if !strings.Contains(run, fmt.Sprintf("anonymous_source_sha256=%x\n", sha256.Sum256([]byte(anonymous)))) {
						t.Fatal("capture detached from anonymous source")
					}
					if route == "anonymous" {
						caught := read(root + "caught-error.txt")
						if !strings.Contains(run, fmt.Sprintf("caught_error_sha256=%x\n", sha256.Sum256([]byte(caught)))) {
							t.Fatal("capture detached from native caught exception")
						}
						parts := strings.SplitN(strings.TrimSuffix(caught, "\n"), "|", 4)
						if len(parts) != 4 || parts[0] != "FLOW_ERROR" || parts[1] != "scalar_missing_id" || parts[2] != "System.FlowException" || strings.TrimSpace(parts[3]) == "" || strings.Count(caught, "\n") != 1 {
							t.Fatalf("invalid native caught exception: %q", caught)
						}
					}
					rows := strings.Split(strings.TrimSuffix(read(root+"org.tsv"), "\n"), "\n")
					if len(rows) != 29 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native TSV")
					}
					want := make(map[string]bool)
					for n, id := range ids {
						row := strings.SplitN(rows[n+1], "\t", 5)
						if len(row) != 5 {
							t.Fatalf("invalid native TSV row: %s", rows[n+1])
						}
						var cell struct {
							Outcome string
							Value   *bool
						}
						decoder := json.NewDecoder(strings.NewReader(row[4]))
						decoder.DisallowUnknownFields()
						if row[0] != id || row[1] != "FLOW_RECORD_BINDING" || row[2] != route || row[3] != "runtime" || decoder.Decode(&cell) != nil || cell.Outcome != "runtime" || cell.Value == nil {
							t.Fatalf("invalid native Boolean cell: %v", row)
						}
						want[id] = *cell.Value
					}
					machine := base.newMachine()
					var execErr error
					if route == "anonymous" {
						analysis := sema.AnalyzeAnonymous(index, anonymous, api)
						program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("native anonymous accepted: %v %v", err, analysis.Diagnostics)
						}
						_, execErr = machine.Execute(program)
					} else {
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, execErr = machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
					}
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" || strings.Count(runtimeErr.ExceptionMessage(), "PROBE|") != 1 {
						t.Fatalf("missing captured assertion transport: %v", execErr)
					}
					_, payload, _ := strings.Cut(runtimeErr.ExceptionMessage(), "PROBE|")
					var got map[string]*bool
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != 28 {
						t.Fatalf("invalid observation: %s (%v)", payload, err)
					}
					countRoute := route
					if route == "istest" {
						countRoute = "isTest"
					}
					for _, id := range ids {
						counts.Run(t, countRoute, id, id, func(t *testing.T) {
							if got[id] == nil || *got[id] != want[id] {
								t.Fatalf("native %v; Glade %v", want[id], got[id])
							}
						})
					}
				})
			}
		})
	}
}
