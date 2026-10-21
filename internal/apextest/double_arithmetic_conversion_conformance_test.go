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

// Owned native arithmetic calls must pass both semantic analysis and execution.
// Decimal.valueOf still rejects an actual Decimal argument in the owned controls.
func TestDoubleArithmeticConversionNativeConformance(t *testing.T) {
	var data struct {
		Fixtures map[string]struct{ Files, SHA256 map[string]string }
		Cells    []struct {
			API, Context, Fixture, ReceiptSHA256, OrgIDHash string
			Native                                          map[string]any
		}
		Carries []string
	}
	raw, err := os.ReadFile("testdata/conformance/double_arithmetic_conversion.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cells) != 6 || len(data.Fixtures) != 3 {
		t.Fatal("incomplete numeric observations")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		t.Run(cell.API+"/"+cell.Context, func(t *testing.T) {
			key := cell.API + "/" + cell.Context
			if seen[key] || len(cell.ReceiptSHA256) != 64 || len(cell.OrgIDHash) != 64 || cell.Fixture != cell.API {
				t.Fatal("unbound numeric observation")
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
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native-accepted arithmetic rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
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
					if _, s, ok := strings.Cut(line, "NATIVE_PROMOTION|"); ok {
						payload = s
					}
				}
			} else if cell.Context == "istest" {
				cases := Discover(index, Options{SelectedClasses: []string{"NumericPromotionProbeTest"}, SelectedMethod: "observe"})
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
				_, payload, ok = strings.Cut(fault.ExceptionMessage(), "NATIVE_PROMOTION|")
				if !ok {
					t.Fatal("missing native marker")
				}
			} else {
				t.Fatal("unknown context")
			}
			var got map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || len(got) != 11 || len(cell.Native) != 11 {
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
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+context] {
				t.Fatal("missing native context")
			}
		}
	}
}
