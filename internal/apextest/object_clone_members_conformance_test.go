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

	"github.com/glade-sh/glade/internal/vm"
)

// Owned source classes distinguish a new root from shared member references,
// including inherited fields and a self-reference. No async drain is involved.
func TestObjectCloneMembersNativeConformance(t *testing.T) {
	var data struct {
		Fixtures map[string]struct{ Files, SHA256 map[string]string }
		Cells    []struct {
			API, Context, Fixture, ReceiptSHA256, OrgIDHash string
			Native                                          map[string]any
		}
		Carries []string
	}
	raw, err := os.ReadFile("testdata/conformance/object_clone_members.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cells) != 6 || len(data.Fixtures) != 3 {
		t.Fatal("incomplete clone observations")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		t.Run(cell.API+"/"+cell.Context, func(t *testing.T) {
			key := cell.API + "/" + cell.Context
			if seen[key] || len(cell.ReceiptSHA256) != 64 || len(cell.OrgIDHash) != 64 || cell.Fixture != cell.API {
				t.Fatal("unbound clone observation")
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
					t.Fatal("source changed: " + name)
				}
				writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = cell.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			payload := ""
			if cell.Context == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(fixture.Files["anonymous.apex"], vm.CompileOptions{APIVersion: cell.API})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, s, ok := strings.Cut(line, "NATIVE_CLONE|"); ok {
						payload = s
					}
				}
			} else if cell.Context == "istest" {
				cases := Discover(index, Options{SelectedClasses: []string{"CloneMemberProbeTest"}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatal("named selection differs")
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatal(methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, err := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var fault *vm.RuntimeError
				if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
					t.Fatalf("missing native observation: %v", err)
				}
				_, payload, ok = strings.Cut(fault.ExceptionMessage(), "NATIVE_CLONE|")
				if !ok {
					t.Fatal("missing native marker")
				}
			} else {
				t.Fatal("unknown context")
			}
			var got map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || len(got) != 13 || len(cell.Native) != 13 {
				t.Fatal("invalid observation: " + payload)
			}
			for name, want := range cell.Native {
				t.Run(name, func(t *testing.T) {
					if !reflect.DeepEqual(got[name], want) {
						t.Fatalf("native=%v; local=%v", want, got[name])
					}
				})
			}
		})
	}
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+context] {
				t.Fatal("missing native context")
			}
		}
	}
}
