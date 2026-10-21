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

func TestSeededIdentityNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/seeded_identity"
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	classes := map[string]string{
		"SeededIdentityProbe":     read("sources/SeededIdentityProbe.cls"),
		"SeededIdentityProbeTest": read("sources/SeededIdentityProbeTest.cls"),
	}
	anonymous := read("sources/anonymous.apex")
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := flowRefsIndex(t, api, classes, nil)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted declarations: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"SeededIdentityProbeTest"}, SelectedMethod: "observe"})
			if len(cases) != 1 {
				t.Fatalf("named methods = %d, want 1", len(cases))
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
					rowCount := 6
					if route == "istest" {
						rowCount = 8
					}
					for name, value := range map[string]string{"api": api, "context": route, "rows": fmt.Sprint(rowCount), "native_complete": "true", "deployment_success": "true", "helper_class_api": api, "wrapper_class_api": api} {
						if !strings.Contains("\n"+run, "\n"+name+"="+value+"\n") {
							t.Fatalf("missing native capture fact: %s", name)
						}
					}
					for name, source := range map[string]string{"SeededIdentityProbe.cls": classes["SeededIdentityProbe"], "anonymous.apex": anonymous} {
						if !strings.Contains(run, fmt.Sprintf("%s_sha256=%x\n", name, sha256.Sum256([]byte(source)))) {
							t.Fatalf("capture detached from %s source", name)
						}
					}
					if route == "istest" && !strings.Contains(run, fmt.Sprintf("SeededIdentityProbeTest.cls_sha256=%x\n", sha256.Sum256([]byte(classes["SeededIdentityProbeTest"])))) {
						t.Fatal("capture detached from named source")
					}
					rows := strings.Split(strings.TrimSuffix(read(root+"org.tsv"), "\n"), "\n")
					if len(rows) != rowCount+1 || rows[0] != "id\troute\tvalue" {
						t.Fatal("incomplete native TSV")
					}
					want := map[string]string{}
					for i, line := range rows[1:] {
						cell := strings.SplitN(line, "\t", 3)
						if len(cell) != 3 || cell[0] != fmt.Sprintf("I%03d", i+1) || cell[1] != route {
							t.Fatalf("invalid native row: %s", line)
						}
						var value string
						if err := json.Unmarshal([]byte(cell[2]), &value); err != nil {
							t.Fatal(err)
						}
						want[cell[0]] = value
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
							t.Fatalf("missing debug transport: %v %v", err, result.Debug)
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
							t.Fatalf("missing assertion transport: %v", err)
						}
						_, payload, _ = strings.Cut(runtimeErr.ExceptionMessage(), "PROBE|")
						// Session validation and implicit runAs insertion are separate
						// boundaries. Preserve their native answers without asserting
						// parity from an identity-seeding change.
						for _, id := range []string{"I007", "I008"} {
							if want[id] == "" {
								t.Fatalf("missing carried native runAs observation %s", id)
							}
							t.Logf("carried %s: native %s", id, want[id])
						}
					}
					var got map[string]string
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != 6 {
						t.Fatalf("invalid observation %s: %v", payload, err)
					}
					for i := 1; i <= 6; i++ {
						id := fmt.Sprintf("I%03d", i)
						t.Run(id, func(t *testing.T) {
							if got[id] != want[id] {
								t.Fatalf("native %q; Glade %q", want[id], got[id])
							}
						})
					}
				})
			}
		})
	}
}
