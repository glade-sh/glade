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
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

func TestQueueableRetryDelayNativeConformance(t *testing.T) {
	var data struct {
		Fixtures map[string]struct{ Files, SHA256 map[string]string }
		Cells    []struct {
			API, Context, Case, Class, Fixture, ReceiptSHA256, OrgIDHash string
			Native                                                       map[string]any
		}
		Carries []string
	}
	raw, err := os.ReadFile("testdata/conformance/retry_delay.json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &data) != nil || len(data.Cells) != 20 || len(data.Fixtures) != 2 {
		t.Fatal("incomplete retry-delay observations")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		t.Run(cell.API+"/"+cell.Context+"/"+cell.Case, func(t *testing.T) {
			key := cell.API + "/" + cell.Context + "/" + cell.Case
			wantFields := 1
			if cell.Case == "chain" {
				wantFields = 6
			}
			if seen[key] || cell.Fixture != cell.API || len(cell.ReceiptSHA256) != 64 || len(cell.OrgIDHash) != 64 || len(cell.Native) != wantFields {
				t.Fatal("unbound observation")
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
			switch cell.Context {
			case "istest":
				cases := Discover(index, Options{SelectedClasses: []string{cell.Class}, SelectedMethod: cell.Case})
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
				_, payload, ok = strings.Cut(fault.ExceptionMessage(), "NATIVE_RETRY_DELAY|")
				if !ok {
					t.Fatal("missing observation marker")
				}
			case "anonymous":
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
				readback := func() string {
					t.Helper()
					result := execute(fixture.Files["readback-"+cell.Case+".apex"])
					for _, line := range result.Debug {
						if _, value, found := strings.Cut(line, "NATIVE_RETRY_DELAY|"); found {
							return value
						}
					}
					t.Fatal("missing anonymous marker")
					return ""
				}
				execute(fixture.Files["enqueue-"+cell.Case+".apex"])
				if err := machine.DrainAsync(&vm.Result{}); err != nil {
					t.Fatal(err)
				}
				if cell.Case == "chain" {
					var first map[string]any
					if json.Unmarshal([]byte(readback()), &first) != nil || len(first) != 2 {
						t.Fatal("anonymous minimum delay was not preserved")
					}
					// Native anonymous receipts wait for the delayed retry.
					// Advance only the explicit local scheduler clock for replay.
					machine.AdvanceDeterministicTime(2 * time.Minute)
					if err := machine.DrainAsync(&vm.Result{}); err != nil {
						t.Fatal(err)
					}
				}
				payload = readback()
			default:
				t.Fatal("unknown context")
			}
			var got map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatal("invalid observation: " + payload)
			}
			for stage, want := range cell.Native {
				t.Run(stage, func(t *testing.T) {
					actual, present := got[stage]
					if !present || !reflect.DeepEqual(actual, want) {
						t.Fatalf("native=%v; present=%v, local=%v", want, present, actual)
					}
				})
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			for _, mode := range []string{"chain", "optionZero", "integerZero", "optionNull", "noOptions"} {
				if !seen[api+"/"+context+"/"+mode] {
					t.Fatal("missing context")
				}
			}
		}
	}
}
