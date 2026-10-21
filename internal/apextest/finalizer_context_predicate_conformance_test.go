package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Native root/body/finalizer predicates distinguish the finalizer's transaction
// from its Queueable parent without relying on event-delivery ordering.
func TestFinalizerContextPredicateNativeConformance(t *testing.T) {
	var data struct {
		Fixtures map[string]struct{ Files, SHA256 map[string]string }
		Cells    []struct {
			API, Context, Fixture, ReceiptSHA256, OrgIDHash string
			Native                                          map[string]map[string]bool
		}
		Carries []string
	}
	raw, err := os.ReadFile("testdata/conformance/finalizer_context_predicate.json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &data) != nil || len(data.Cells) != 6 || len(data.Fixtures) != 3 {
		t.Fatal("incomplete finalizer observations")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		t.Run(cell.API+"/"+cell.Context, func(t *testing.T) {
			key := cell.API + "/" + cell.Context
			if seen[key] || cell.Fixture != cell.API || len(cell.ReceiptSHA256) != 64 || len(cell.OrgIDHash) != 64 {
				t.Fatal("unbound finalizer row")
			}
			seen[key] = true
			fixture, ok := data.Fixtures[cell.Fixture]
			if !ok {
				t.Fatal("missing fixture")
			}
			var project struct{ SourceAPIVersion string }
			if json.Unmarshal([]byte(fixture.Files["sfdx-project.json"]), &project) != nil || project.SourceAPIVersion != cell.API {
				t.Fatal("source API differs")
			}
			root := t.TempDir()
			for name, source := range fixture.Files {
				sum := sha256.Sum256([]byte(source))
				if hex.EncodeToString(sum[:]) != fixture.SHA256[name] {
					t.Fatal("changed source: " + name)
				}
				writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			payload := ""
			if cell.Context == "istest" {
				cases := Discover(index, Options{SelectedClasses: []string{"FinalizerPredicateProbeTest"}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatal("named selection differs")
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatal(methodErrors[key], programErrors[key])
				}
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var fault *vm.RuntimeError
				if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
					t.Fatalf("missing native transport: %v", err)
				}
				_, payload, ok = strings.Cut(fault.ExceptionMessage(), "NATIVE_FINALIZER|")
				if !ok {
					t.Fatal("missing native marker")
				}
			} else if cell.Context == "anonymous" {
				execute := func(source string) vm.Result {
					t.Helper()
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: cell.API})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					return result
				}
				execute(fixture.Files["enqueue.apex"])
				if err := machine.DrainAsync(&vm.Result{}); err != nil {
					t.Fatal(err)
				}
				result := execute(fixture.Files["readback.apex"])
				for _, line := range result.Debug {
					if _, value, found := strings.Cut(line, "NATIVE_FINALIZER|"); found {
						payload = value
					}
				}
			} else {
				t.Fatal("unknown context")
			}
			var got map[string]map[string]bool
			if json.Unmarshal([]byte(payload), &got) != nil || len(cell.Native) != 7 {
				t.Fatal("invalid observation: " + payload)
			}
			for _, stage := range []string{"root", "body0", "body1", "body2", "finalizer0", "finalizer1", "finalizer2"} {
				flags, ok := cell.Native[stage]
				if !ok || len(flags) != 5 {
					t.Fatal("missing native stage")
				}
				for _, flag := range []string{"queueable", "future", "batch", "scheduled", "test"} {
					want, ok := flags[flag]
					if !ok {
						t.Fatal("missing native flag")
					}
					t.Run(stage+"/"+flag, func(t *testing.T) {
						actual, present := got[stage][flag]
						if !present || actual != want {
							t.Fatalf("native=%v; present=%v, local=%v", want, present, actual)
						}
					})
				}
			}
			if len(got) > 7 {
				t.Fatal("unexpected async stages")
			}
		})
	}
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+context] {
				t.Fatal("missing context")
			}
		}
	}
}
