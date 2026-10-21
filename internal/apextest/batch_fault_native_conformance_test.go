package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// The corrected witness observes ingress before subscriber queries. Earlier
// zero-event answers are withdrawn and are not inputs to this test.
func TestBatchFaultNativeConformance(t *testing.T) {
	var data struct {
		APIVersions []string `json:"apiVersions"`
		Fixtures    map[string]struct {
			Classes, Objects, Triggers map[string]string
			SourceSHA256               map[string]string `json:"sourceSHA256"`
		} `json:"fixtures"`
		Cells []struct {
			ID, API, Route, Source, ClassName, Method, EnqueueSource, SourceSHA256, NativeSHA256 string
			Native                                                                               json.RawMessage
		} `json:"cells"`
		Controls []json.RawMessage                                 `json:"controls"`
		Carries  []struct{ ID, API, Route, Status, Reason string } `json:"carries"`
	}
	raw, err := os.ReadFile("testdata/conformance/batch_fault_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || len(data.Cells) != 40 || len(data.Controls) != 24 || len(data.Carries) != 2 {
		t.Fatal("incomplete batch observation matrix")
	}
	checkHash := func(source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != want {
			t.Fatal("owned source changed")
		}
	}
	for _, carry := range data.Carries {
		if (carry.ID != "C13-08" && carry.ID != "C13-10") || carry.API != "62.0" || carry.Route != "anonymous" || carry.Status != "withdrawn-no-answer" || carry.Reason == "" {
			t.Fatal("invalid unanswered native cell")
		}
		t.Logf("carried %s/%s/%s: %s", carry.API, carry.Route, carry.ID, carry.Reason)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Route + "/" + cell.ID
		if seen[key] || len(cell.NativeSHA256) != 64 {
			t.Fatal("duplicate/unbound native cell")
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			checkHash(cell.Source, cell.SourceSHA256)
			fixture := data.Fixtures[cell.API]
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, cell.API))
			for _, group := range []struct {
				kind, dir, extension, meta string
				sources                    map[string]string
			}{
				{"classes", "classes", ".cls", "ApexClass", fixture.Classes},
				{"objects", "objects", ".object-meta.xml", "", fixture.Objects},
				{"triggers", "triggers", ".trigger", "ApexTrigger", fixture.Triggers},
			} {
				for name, source := range group.sources {
					checkHash(source, fixture.SourceSHA256[group.kind+"/"+name])
					path := filepath.Join(root, "force-app/main/default", group.dir, name+group.extension)
					if group.kind == "objects" {
						path = filepath.Join(root, "force-app/main/default/objects", name, name+group.extension)
					}
					writeFile(t, path, source)
					if group.meta != "" {
						writeFile(t, path+"-meta.xml", "<"+group.meta+"><apiVersion>"+cell.API+"</apiVersion></"+group.meta+">")
					}
				}
			}
			if cell.Route == "istest" {
				path := filepath.Join(root, "force-app/main/default/classes", cell.ClassName+".cls")
				writeFile(t, path, cell.Source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+cell.API+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native setup accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runtime := CompileProjectRuntimeForRequest(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			machine := runner.newMachine()
			execute := func(source string, inClass bool) error {
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: cell.API})
				if err != nil {
					t.Fatal(err)
				}
				if inClass {
					_, err = machine.ExecuteInClass(program, cell.ClassName)
				} else {
					_, err = machine.Execute(program)
				}
				return err
			}
			var execErr error
			if cell.Route == "anonymous" {
				seed := "insert new Account(Name='E51_OWNER_C51A" + cell.API[:2] + "');" + cell.EnqueueSource
				if err := execute(seed, false); err != nil {
					t.Fatal(err)
				}
				// The capture observes the job in a later transaction, independent of the
				// enqueue response. Drain local work before evaluating that same readback.
				_ = machine.DrainAsync(&vm.Result{})
				execErr = execute(cell.Source, false)
			} else {
				machine.EnableTestContext()
				if cell.Method == "captureSeed" {
					execErr = execute(cell.Method+"();", true)
				} else {
					cases := Discover(index, Options{SelectedClasses: []string{cell.ClassName}, SelectedMethod: cell.Method})
					if len(cases) != 1 {
						t.Fatalf("named discovery: %d", len(cases))
					}
					methods, methodErrors := compileTestMethods(cases)
					programs, programErrors := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("named compilation: %v %v", methodErrors[key], programErrors[key])
					}
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					_, execErr = machine.ExecuteInClass(programs[key], cell.ClassName)
				}
			}
			var fault *vm.RuntimeError
			if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
				t.Fatalf("missing observation: %v", execErr)
			}
			_, payload, found := strings.Cut(fault.ExceptionMessage(), "C126_RESULT|"+cell.ID+"|")
			if !found {
				t.Fatalf("missing native marker: %v", execErr)
			}
			var got, want map[string]any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(cell.Native, &want); err != nil {
				t.Fatal(err)
			}
			// The desk redacts only generated job identity. Require one local identity
			// shared by the job, event and ingress witnesses before projecting it.
			job, _ := got["job"].(string)
			if job != "" {
				if len(job) != 15 && len(job) != 18 || !strings.HasPrefix(job, "707") {
					t.Fatalf("invalid local job identity: %q", job)
				}
				payload = strings.ReplaceAll(payload, `"`+job+`"`, `"[ID]"`)
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatal(err)
				}
			}
			// The desk derives effectProof from the returned stop exception; it
			// is not an Apex measurement field. Assert that proof before comparison.
			if proof, exists := want["effectProof"]; exists {
				if proof != "matching-stop-exception" || got["stopType"] != want["stopType"] || got["stopMessage"] != want["stopMessage"] {
					t.Fatalf("native completion proof missing: %s", payload)
				}
				delete(want, "effectProof")
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native %s; Glade %s", cell.Native, payload)
			}
		})
	}
}
