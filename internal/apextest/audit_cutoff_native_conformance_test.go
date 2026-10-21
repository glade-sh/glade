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

func TestAuditCutoffNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/audit_cutoff"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	classes := map[string]string{"MeadowAuditProbe": read("sources/MeadowAuditProbe.cls"), "MeadowAuditProbeTest": read("sources/MeadowAuditProbeTest.cls")}
	anonymous := read("sources/anonymous.apex")
	sources := map[string]string{"helper": classes["MeadowAuditProbe"], "wrapper": classes["MeadowAuditProbeTest"], "anonymous": anonymous}
	ids := []string{"AT01", "AT02", "AT03", "AC01"}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "AuditCutoff", api)
			index := nativeBoundaryIndex(t, api, classes, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"MeadowAuditProbeTest"}, SelectedMethod: "observeRows"})
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
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					root := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
					run := read(root + "RUN-FACTS.txt")
					for name, value := range map[string]string{"api": api, "context": route, "rows": "4", "native_complete": "true", "deployment_success": "true", "helper_class_api": api, "wrapper_class_api": api} {
						if !strings.Contains("\n"+run, "\n"+name+"="+value+"\n") {
							t.Fatalf("missing native capture fact: %s", name)
						}
					}
					for name, source := range sources {
						if !strings.Contains(run, fmt.Sprintf("%s_source_sha256=%x\n", name, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture detached from exact %s source", name)
						}
					}
					rows := strings.Split(strings.TrimSuffix(read(root+"org.tsv"), "\n"), "\n")
					if len(rows) != 5 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native TSV")
					}
					want := make(map[string]bool)
					for i, id := range ids {
						row, group := strings.SplitN(rows[i+1], "\t", 5), "AUDIT"
						if len(row) != 5 {
							t.Fatalf("invalid native TSV row: %s", rows[i+1])
						}
						if id == "AC01" {
							group = "CONTROL"
						}
						var cell struct {
							Outcome string
							Value   *bool
						}
						decoder := json.NewDecoder(strings.NewReader(row[4]))
						decoder.DisallowUnknownFields()
						if row[0] != id || row[1] != group || row[2] != route || row[3] != "runtime" || decoder.Decode(&cell) != nil || cell.Outcome != "runtime" || cell.Value == nil {
							t.Fatalf("invalid native Boolean cell: %v", row)
						}
						want[id] = *cell.Value
					}
					machine, payload := base.newMachine(), ""
					if route == "anonymous" {
						analysis := sema.AnalyzeAnonymous(index, anonymous, api)
						program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("native anonymous accepted: %v %v", err, analysis.Diagnostics)
						}
						result, err := machine.Execute(program)
						if err != nil || len(result.Debug) != 1 || !strings.HasPrefix(result.Debug[0], "PROBE|") {
							t.Fatalf("missing successful debug transport: %v %v", err, result.Debug)
						}
						payload = strings.TrimPrefix(result.Debug[0], "PROBE|")
					} else {
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, err := machine.ExecuteTestInClass(programs[key], cases[0].ClassName)
						var runtimeErr *vm.RuntimeError
						if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.AssertException" || strings.Count(runtimeErr.ExceptionMessage(), "PROBE|") != 1 {
							t.Fatalf("missing captured named assertion transport: %v", err)
						}
						_, payload, _ = strings.Cut(runtimeErr.ExceptionMessage(), "PROBE|")
					}
					var got map[string]*bool
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != 4 {
						t.Fatalf("invalid observation: %s (%v)", payload, err)
					}
					countRoute := route
					if route == "istest" {
						countRoute = "isTest"
					}
					for _, id := range ids {
						counts.Run(t, countRoute, id, id, func(t *testing.T) {
							if got[id] == nil {
								t.Fatal("missing observed Boolean")
							}
							if *got[id] != want[id] {
								t.Fatalf("native %v; Glade %v", want[id], *got[id])
							}
						})
					}
				})
			}
		})
	}
}
