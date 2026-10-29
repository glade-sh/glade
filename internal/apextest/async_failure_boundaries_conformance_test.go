package apextest

import (
	"crypto/sha256"
	"encoding/hex"
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

// Owned API 62/67 observations distinguish named stopTest effects from real
// async rollback. No project logger or application-specific subscriber is used.
func TestAsyncFailureBoundariesNativeConformance(t *testing.T) {
	var data struct {
		APIVersions []string                                             `json:"apiVersions"`
		Fixtures    map[string]struct{ Files, SHA256 map[string]string } `json:"fixtures"`
		Cells       []struct {
			ID, API, Route, Fixture, Method, Enqueue, Readback string
			Native                                             json.RawMessage
			ReceiptSHA256                                      string
		} `json:"cells"`
		Carries []string `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/async_failure_boundaries.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.Cells) != 32 || len(data.Fixtures) != 4 {
		t.Fatal("incomplete async boundary matrix")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route + "/" + cell.ID
		if seen[key] || len(cell.ReceiptSHA256) != 64 {
			t.Fatal("duplicate or unbound native cell")
		}
		seen[key] = true
		runConformanceSubtestParallel(t, key, func(t *testing.T) {
			fixture, ok := data.Fixtures[cell.Fixture]
			if !ok || cell.Fixture != cell.API+"/"+cell.Route {
				t.Fatal("missing native fixture")
			}
			var project struct{ SourceAPIVersion string }
			if json.Unmarshal([]byte(fixture.Files["sfdx-project.json"]), &project) != nil || project.SourceAPIVersion != cell.API {
				t.Fatal("native source API mismatch")
			}
			root := t.TempDir()
			for name, source := range fixture.Files {
				sum := sha256.Sum256([]byte(source))
				if hex.EncodeToString(sum[:]) != fixture.SHA256[name] {
					t.Fatal("owned source changed: " + name)
				}
				writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted fixture: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runtime := CompileProjectRuntimeForRequest(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			machine := runner.newMachine()
			execute := func(source string) (vm.Result, error) {
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: cell.API})
				if err != nil {
					t.Fatal(err)
				}
				return machine.Execute(program)
			}
			payload := ""
			if cell.Route == "istest" {
				cases := Discover(index, Options{SelectedClasses: []string{"AsyncBoundaryProbeTest"}, SelectedMethod: cell.Method})
				if len(cases) != 1 {
					t.Fatalf("native method discovery: %v", cases)
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				caseKey := testCaseKey(cases[0])
				if methodErrors[caseKey] != nil || programErrors[caseKey] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[caseKey], programErrors[caseKey])
				}
				if err := machine.RegisterMethod(methods[caseKey]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[caseKey], cases[0].ClassName)
				var fault *vm.RuntimeError
				if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
					t.Fatalf("missing native observation: %v", execErr)
				}
				_, payload, ok = strings.Cut(fault.ExceptionMessage(), "NATIVE_ASYNC|"+cell.ID+"|")
				if !ok {
					t.Fatalf("missing marker: %v", execErr)
				}
			} else if cell.Route == "anonymous" {
				if _, err := execute(cell.Enqueue); err != nil {
					t.Fatal(err)
				}
				// Real async transactions continue independently after a failed job.
				// A failed DrainAsync consumes that job; resume the remaining queue.
				drained := false
				for i := 0; i < 10; i++ {
					if err := machine.DrainAsync(&vm.Result{}); err == nil {
						drained = true
						break
					}
				}
				if !drained {
					t.Fatal("async queue did not finish")
				}
				result, err := execute(cell.Readback)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, found := strings.Cut(line, "NATIVE_ASYNC|"+cell.ID+"|"); found {
						payload = value
					}
				}
			} else {
				t.Fatal("unknown captured route")
			}
			var got, want any
			if payload == "" || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal(cell.Native, &want) != nil {
				t.Fatalf("invalid observation %q", payload)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; local=%s", cell.Native, payload)
			}
		})
	}
}
