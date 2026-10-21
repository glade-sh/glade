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

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These captures reference mutation APIs only behind an explicit mock branch.
// They establish source visibility, not hosted credential-management behavior.
func TestCredentialMockSourceVisibilityNativeConformance(t *testing.T) {
	var data struct {
		NativeObservationCount int
		APIVersions            []string
		Rows                   []struct {
			ID, API, Context, Outcome, OrgIDHash, Operation            string
			Test, Anonymous, AnonymousSHA256, NativeMessage, RawSHA256 string
			Sources, SourceSHA256                                      map[string]string
			Value                                                      map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/credential_mock_visibility.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.NativeObservationCount != 36 || len(data.Rows) != 36 || strings.Join(data.APIVersions, ",") != "62.0,65.0,66.0,67.0" {
		t.Fatal("native source/context inventory differs")
	}
	hashMatches := func(source, hash string) bool {
		sum := sha256.Sum256([]byte(source))
		return hex.EncodeToString(sum[:]) == hash
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.API + "/" + row.Context + "/" + row.ID
		t.Run(key, func(t *testing.T) {
			if seen[key] || len(row.OrgIDHash) != 64 || len(row.RawSHA256) != 64 || row.Operation == "" {
				t.Fatal("duplicate or unbound native cell")
			}
			seen[key] = true
			root := t.TempDir()
			project, _ := json.Marshal(map[string]any{"sourceApiVersion": row.API, "packageDirectories": []map[string]any{{"path": "force-app", "default": true}}})
			writeFile(t, filepath.Join(root, "sfdx-project.json"), string(project))
			for name, source := range row.Sources {
				if !hashMatches(source, row.SourceSHA256[name]) {
					t.Fatal("captured source changed")
				}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+row.API+"</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if row.Context == "anonymous" {
				if !hashMatches(row.Anonymous, row.AnonymousSHA256) {
					t.Fatal("captured anonymous source changed")
				}
				analysis = sema.AnalyzeAnonymous(index, row.Anonymous, row.API)
			}
			if row.Outcome == "compile-rejected" {
				if row.Value["compiled"] != false || !analysis.HasErrors() {
					t.Fatal("native compile rejection differs")
				}
				for _, d := range analysis.Diagnostics {
					if d.Severity == diagnostic.Error && strings.Contains(d.Message, row.Operation) {
						t.Logf("native rejection %v; local diagnostic %s", row.Value, d.Message)
						return
					}
				}
				t.Fatalf("no rejection for captured call %s: %v", row.Operation, analysis.Diagnostics)
			}
			if row.Outcome != "runtime" || index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native mock executes; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = row.API
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var execErr error
			if row.Context == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = runner.execute(program)
			} else if row.Context == "isTest" {
				cases := Discover(index, Options{SelectedClasses: []string{row.Test}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatal("native method discovery differs")
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
				t.Fatal("unexpected native context")
			}
			var runtimeErr *vm.RuntimeError
			if !errors.As(execErr, &runtimeErr) || stubNamespaceExceptionType(runtimeErr.Type) != "System.AssertException" {
				t.Fatalf("missing mock observation: %v", execErr)
			}
			decode := func(message string) map[string]any {
				t.Helper()
				_, payload, found := strings.Cut(message, ", Actual: ")
				if !found || !strings.Contains(message, "CREDENTIAL_MOCK_OBSERVATION") {
					t.Fatal("observation transport differs: ", message)
				}
				var value map[string]any
				if err := json.Unmarshal([]byte(payload), &value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			if !reflect.DeepEqual(row.Value, decode(row.NativeMessage)) || !reflect.DeepEqual(row.Value, decode(runtimeErr.ExceptionMessage())) {
				t.Fatalf("native %v; local %s", row.Value, runtimeErr.ExceptionMessage())
			}
		})
	}
	for _, api := range data.APIVersions {
		for _, context := range []string{"anonymous", "isTest"} {
			for _, operation := range []string{"update", "delete", "update_external", "delete_external"} {
				if !seen[api+"/"+context+"/"+operation] {
					t.Fatal("missing native source cell")
				}
			}
		}
	}
	for _, api := range []string{"65.0", "67.0"} {
		for _, id := range []string{"update_wrong_arity", "update_wrong_input"} {
			if !seen[api+"/isTest/"+id] {
				t.Fatal("missing native invalid-signature control")
			}
		}
	}
}

// This is the retained local service boundary, not a native hosted assertion.
func TestCredentialMutationsRemainOffline(t *testing.T) {
	calls := map[string]string{
		"updateNamedCredential":    "'OwnedAlias',new ConnectApi.NamedCredentialInput()",
		"deleteNamedCredential":    "'OwnedAlias'",
		"updateExternalCredential": "'OwnedAlias',new ConnectApi.ExternalCredentialInput()",
		"deleteExternalCredential": "'OwnedAlias'",
	}
	for _, api := range []string{"62.0", "65.0", "66.0", "67.0"} {
		for method, args := range calls {
			t.Run(api+"/"+method, func(t *testing.T) {
				callee := "ConnectApi.NamedCredentials." + method
				program, err := vm.CompileAnonymousWithOptions(callee+"("+args+");", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = vm.New(nil).Execute(program)
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "ConnectApi.ConnectApiException" || runtimeErr.ExceptionMessage() != callee+" is not supported in local tests" {
					t.Fatalf("offline service boundary changed: %v", err)
				}
			})
		}
	}
}
