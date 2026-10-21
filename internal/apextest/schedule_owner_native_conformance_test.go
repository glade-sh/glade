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

func TestScheduleOwnerNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/schedule_owner"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	classes := map[string]string{"MeadowScheduleOwnerProbe": read("sources/MeadowScheduleOwnerProbe.cls"), "MeadowScheduleOwnerProbeTest": read("sources/MeadowScheduleOwnerProbeTest.cls"), "MeadowScheduleOwnerBatch": read("sources/MeadowScheduleOwnerBatch.cls")}
	anonymous := read("sources/anonymous.apex")
	sources := map[string]string{"helper": classes["MeadowScheduleOwnerProbe"], "wrapper": classes["MeadowScheduleOwnerProbeTest"], "anonymous": anonymous, "batch": classes["MeadowScheduleOwnerBatch"]}
	baseIDs := []string{"SO01", "SO02", "SB01", "SC01", "SC02", "SC03"}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "ScheduleOwner", api)
			index := nativeBoundaryIndex(t, api, classes, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"MeadowScheduleOwnerProbeTest"}, SelectedMethod: "observe"})
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
					ids := append([]string(nil), baseIDs...)
					if route == "istest" {
						ids = append(ids, "SR01", "SR02", "SC04")
					}
					root := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
					run := read(root + "RUN-FACTS.txt")
					for name, value := range map[string]string{"api": api, "context": route, "rows": fmt.Sprint(len(ids)), "native_complete": "true", "deployment_success": "true", "helper_class_api": api, "wrapper_class_api": api, "batch_class_api": api} {
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
					if len(rows) != len(ids)+1 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
						t.Fatal("incomplete native TSV")
					}
					want := make(map[string]bool)
					for i, id := range ids {
						row, group := strings.SplitN(rows[i+1], "\t", 5), "SCHEDULE_OWNER"
						if len(row) != 5 {
							t.Fatalf("invalid native TSV row: %s", rows[i+1])
						}
						if strings.HasPrefix(id, "SC") {
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
						var runtimeErr *vm.RuntimeError
						if errors.As(err, &runtimeErr) && runtimeErr.Type == "UnsupportedFeature" && runtimeErr.Message == `unsupported call "System.abortJob local async scheduling surface"` {
							for _, id := range ids {
								counts.RunKind(t, route, id, id, "carried", func(t *testing.T) {
									value, found := want[id]
									if !found {
										t.Fatal("missing answered native carry cell")
									}
									t.Logf("native %s=%v; carried: exact anonymous cleanup reaches the existing scheduling guard before its payload: %s", id, value, runtimeErr.Message)
								})
							}
							return
						}
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
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != len(ids) {
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
