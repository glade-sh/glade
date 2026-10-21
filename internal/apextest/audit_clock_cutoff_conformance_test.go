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

// Native audit-date cutoffs cover ordinary inserts, a stateful QueryLocator
// batch, and an explicit future date that must not advance the runtime clock.
func TestAuditClockCutoffNativeConformance(t *testing.T) {
	var data struct {
		NativeObservationCount int
		SourceAPIs             []string
		Rows                   []struct {
			ID, SourceAPI, TestAPI, ProjectAPI, TransportAPI, Context, OrgIDHash string
			Test, Method, Anonymous, AnonymousSHA256, NativeMessage, RawSHA256   string
			Sources, SourceAPIs, SourceSHA256                                    map[string]string
			Value                                                                map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/audit_clock_cutoff.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.NativeObservationCount != 12 || len(data.Rows) != 12 || strings.Join(data.SourceAPIs, ",") != "62.0,65.0,67.0" {
		t.Fatal("native scope differs")
	}
	checkHash := func(source, hash string) bool {
		sum := sha256.Sum256([]byte(source))
		return hex.EncodeToString(sum[:]) == hash
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.SourceAPI + "/" + row.Context + "/" + row.ID
		t.Run(key, func(t *testing.T) {
			if seen[key] || len(row.OrgIDHash) != 64 || len(row.RawSHA256) != 64 || row.SourceAPI != row.TransportAPI {
				t.Fatal("invalid or repeated native cell")
			}
			seen[key] = true
			projectAPI := row.SourceAPI
			if row.SourceAPI == "65.0" {
				projectAPI = "66.0"
			}
			if row.ProjectAPI != projectAPI || (row.Context == "isTest" && row.TestAPI != projectAPI) {
				t.Fatal("native source/project version differs")
			}
			root := t.TempDir()
			project, _ := json.Marshal(map[string]any{"sourceApiVersion": row.ProjectAPI, "packageDirectories": []map[string]any{{"path": "force-app", "default": true}}})
			writeFile(t, filepath.Join(root, "sfdx-project.json"), string(project))
			for name, source := range row.Sources {
				if !checkHash(source, row.SourceSHA256[name]) {
					t.Fatal("captured source differs")
				}
				version := row.SourceAPI
				if name == row.Test {
					version = row.TestAPI
				}
				if row.SourceAPIs[name] != version {
					t.Fatal("captured class version differs")
				}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+version+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native executes; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = row.ProjectAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var execErr error
			if row.Context == "anonymous" {
				if !checkHash(row.Anonymous, row.AnonymousSHA256) {
					t.Fatal("captured anonymous source differs")
				}
				program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: row.SourceAPI})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = runner.execute(program)
			} else if row.Context == "isTest" {
				cases := Discover(index, Options{SelectedClasses: []string{row.Test}, SelectedMethod: row.Method})
				if len(cases) != 1 {
					t.Fatal("native method selection differs")
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
				_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
			} else {
				t.Fatal("unknown native context")
			}
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || stubNamespaceExceptionType(runtimeErr.Type) != "System.AssertException" {
				t.Fatalf("missing observation transport: %v", execErr)
			}
			decode := func(message string) map[string]any {
				t.Helper()
				_, payload, found := strings.Cut(message, ", Actual: ")
				if !found || !strings.Contains(message, "AUDIT_CUTOFF_OBSERVATION") {
					t.Fatal("native observation envelope differs: ", message)
				}
				var value map[string]any
				if err := json.Unmarshal([]byte(payload), &value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			if !reflect.DeepEqual(row.Value, decode(row.NativeMessage)) || !reflect.DeepEqual(row.Value, decode(runtimeErr.ExceptionMessage())) {
				t.Fatalf("native %v; Glade %s", row.Value, runtimeErr.ExceptionMessage())
			}
		})
	}
	for _, api := range data.SourceAPIs {
		for _, suffix := range []string{"anonymous/sequential", "isTest/sequential", "isTest/batch", "isTest/future"} {
			if !seen[api+"/"+suffix] {
				t.Fatal("missing native cell ", api+"/"+suffix)
			}
		}
	}
}
