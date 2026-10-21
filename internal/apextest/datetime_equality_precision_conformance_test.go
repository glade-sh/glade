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

// Captured source metadata keeps legacy endpoint initialization and assertion
// calls at API 54. API 67 custom-object access rejections remain explicit
// carries; they are not timestamp observations or local permission assertions.
func TestDatetimeEqualityPrecisionNativeConformance(t *testing.T) {
	var data struct {
		NativeObservationCount, CarriedCount int
		Captures                             []struct {
			ID, TransportAPI, Route, TestClass string
			Files, FileSHA256                  map[string]string
			CheckOnly, RollbackOnError         bool
			Rows                               []struct {
				Method, Kind, CarryReason, CarryOwner string
				Native                                json.RawMessage
			}
		}
	}
	raw, err := os.ReadFile("testdata/conformance/datetime_equality_precision.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.NativeObservationCount != 41 || data.CarriedCount != 3 || len(data.Captures) != 7 {
		t.Fatal("incomplete named DateTime precision observations")
	}
	observations, carries := 0, 0
	for _, capture := range data.Captures {
		t.Run(capture.ID, func(t *testing.T) {
			if !capture.CheckOnly || !capture.RollbackOnError || capture.Route != "istest" || len(capture.Files) != len(capture.FileSHA256) {
				t.Fatal("invalid capture provenance")
			}
			root := t.TempDir()
			for path, source := range capture.Files {
				digest := sha256.Sum256([]byte(source))
				if hex.EncodeToString(digest[:]) != capture.FileSHA256[path] {
					t.Fatalf("captured source changed: %s", path)
				}
				writeFile(t, filepath.Join(root, path), source)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatal("owned named source or metadata rejected")
			}
			cases := Discover(index, Options{SelectedClasses: []string{capture.TestClass}})
			if len(cases) != len(capture.Rows) {
				t.Fatalf("named cases=%d, native rows=%d", len(cases), len(capture.Rows))
			}
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			org.APIVersion = capture.TransportAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			counts := newConformanceCountLog(t, "DatetimeEqualityPrecision", capture.ID)
			for _, row := range capture.Rows {
				observations++
				kind := "exact"
				if row.CarryReason != "" {
					kind = "carried"
					carries++
				}
				counts.RunKind(t, "isTest", row.Method, row.Method, kind, func(t *testing.T) {
					if row.CarryReason != "" {
						var native map[string]any
						if err := json.Unmarshal(row.Native, &native); err != nil || native["exceptionType"] != "System.DmlException" || row.CarryOwner == "" {
							t.Fatal("missing carried native rejection")
						}
						t.Logf("carried to %s: %s; native=%s", row.CarryOwner, row.CarryReason, row.Native)
						return
					}
					for _, test := range cases {
						if test.MethodName != row.Method {
							continue
						}
						key := testCaseKey(test)
						if methodErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						machine.EnableTestContext()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						_, execErr := machine.ExecuteInClass(programs[key], test.ClassName)
						if row.Kind == "outcome" {
							var native struct{ Status, Message string }
							if err := json.Unmarshal(row.Native, &native); err != nil {
								t.Fatal(err)
							}
							if native.Status == "pass" {
								if execErr != nil {
									t.Fatalf("native returned: %v", execErr)
								}
								return
							}
							var runtimeErr *vm.RuntimeError
							if native.Status != "fail" || !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" || runtimeErr.ExceptionMessage() != strings.TrimPrefix(native.Message, "System.AssertException: ") {
								t.Fatalf("native=%s; local=%v", row.Native, execErr)
							}
							return
						}
						var runtimeErr *vm.RuntimeError
						if row.Kind != "observation" || !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
							t.Fatalf("missing terminal observation: %v", execErr)
						}
						_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "Actual: ")
						var want, got any
						if !found || json.Unmarshal(row.Native, &want) != nil || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(want, got) {
							t.Fatalf("native=%s; local=%s", row.Native, payload)
						}
						return
					}
					t.Fatalf("native method not discovered: %s", row.Method)
				})
			}
		})
	}
	if observations != data.NativeObservationCount || carries != data.CarriedCount {
		t.Fatalf("observations=%d, carries=%d", observations, carries)
	}
}
