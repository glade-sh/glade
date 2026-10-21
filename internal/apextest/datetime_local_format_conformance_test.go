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

// Owned native observations cover local pattern formatting, historical offsets,
// zone labels and fractional-second preservation. Anonymous cells are UTC controls.
func TestDatetimeLocalFormatNativeConformance(t *testing.T) {
	var data struct {
		Captures []struct {
			API, Route, ReceiptSHA256 string
			Files, SHA256             map[string]string
			Anonymous                 string
			Native                    json.RawMessage
		}
		Carries []string
	}
	raw, err := os.ReadFile("testdata/conformance/datetime_local_format.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Captures) != 4 {
		t.Fatal("incomplete native local format matrix")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, capture := range data.Captures {
		key := capture.API + "/" + capture.Route
		if seen[key] || (capture.API != "62.0" && capture.API != "67.0") || len(capture.ReceiptSHA256) != 64 {
			t.Fatal("unbound native capture")
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			for name, source := range capture.Files {
				digest := sha256.Sum256([]byte(source))
				if hex.EncodeToString(digest[:]) != capture.SHA256[name] {
					t.Fatal("captured source changed: " + name)
				}
				writeFile(t, filepath.Join(root, filepath.FromSlash(name)), source)
			}
			var metadata struct{ SourceAPIVersion string }
			if json.Unmarshal([]byte(capture.Files["sfdx-project.json"]), &metadata) != nil || metadata.SourceAPIVersion != capture.API {
				t.Fatal("source API mismatch")
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("native accepted source rejected")
			}
			org := orgFromIndex(index)
			org.APIVersion = capture.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			payload := ""
			if capture.Route == "istest" {
				cases := Discover(index, Options{SelectedClasses: []string{"LocalZoneNativeProbeTest"}, SelectedMethod: "collect"})
				if len(cases) != 1 {
					t.Fatal("named observation not discovered")
				}
				methods, methodErrors := compileTestMethodsForIndex(&index, cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				key := testCaseKey(cases[0])
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
				}
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				machine.EnableTestContext()
				_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
				var fault *vm.RuntimeError
				if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
					t.Fatalf("missing named observation: %v", execErr)
				}
				_, payload, _ = strings.Cut(fault.ExceptionMessage(), "NATIVE_FORMAT|")
			} else if capture.Route == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(capture.Anonymous, vm.CompileOptions{APIVersion: capture.API})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, ok := strings.Cut(line, "NATIVE_FORMAT|"); ok {
						payload = value
					}
				}
			} else {
				t.Fatal("unknown native route")
			}
			var got, want []any
			if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal(capture.Native, &want) != nil {
				t.Fatalf("invalid observation: %q", payload)
			}
			count := 2
			if capture.Route == "istest" {
				count = 8
			}
			if len(want) != count || !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; local=%s", capture.Native, payload)
			}
		})
	}
}
