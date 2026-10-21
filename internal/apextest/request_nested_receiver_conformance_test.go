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

func TestRequestNestedReceiverNativeConformance(t *testing.T) {
	var data struct {
		SourceAPI, ProjectAPI, TransportAPI, OrgIDHash string
		NativeObservationCount                         int
		Rows                                           []struct {
			ID, API, Context, Outcome, Test, Anonymous, AnonymousSHA256, NativeMessage, RawSHA256 string
			Sources, SourceSHA256                                                                 map[string]string
			Value                                                                                 map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/request_nested_receiver.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.SourceAPI != "65.0" || data.ProjectAPI != "66.0" || data.TransportAPI != "65.0" || data.NativeObservationCount != 5 || len(data.Rows) != 5 {
		t.Fatal("native source/context scope differs")
	}
	checkHash := func(text, hash string) bool {
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:]) == hash
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		t.Run(row.ID+"/"+row.Context, func(t *testing.T) {
			key := row.ID + "/" + row.Context
			if seen[key] || row.API != data.SourceAPI || len(row.RawSHA256) != sha256.Size*2 || len(data.OrgIDHash) != sha256.Size*2 {
				t.Fatal("invalid or duplicate native cell")
			}
			seen[key] = true
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"66.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name, source := range row.Sources {
				if !checkHash(source, row.SourceSHA256[name]) {
					t.Fatal("captured source differs")
				}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>65.0</apiVersion></ApexClass>")
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if row.Outcome == "compile-rejected" {
				if row.Value["compiled"] != false || !analysis.HasErrors() {
					t.Fatal("native rejects the lexical Request declaration")
				}
				for _, d := range analysis.Diagnostics {
					if d.Code == "GLADESEMA008" && d.Severity == diagnostic.Error && strings.Contains(d.Message, "Request.getCurrent") {
						return
					}
				}
				t.Fatalf("missing member rejection differs: %v", analysis.Diagnostics)
			}
			if row.Outcome != "runtime" || index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native executes; Glade diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = data.ProjectAPI
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var execErr error
			if row.Context == "anonymous" {
				if !checkHash(row.Anonymous, row.AnonymousSHA256) {
					t.Fatal("captured anonymous source differs")
				}
				program, err := vm.CompileAnonymousWithOptions(row.Anonymous, vm.CompileOptions{APIVersion: row.API})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr = runner.execute(program)
			} else if row.Context == "isTest" {
				cases := Discover(index, Options{SelectedClasses: []string{row.Test}, SelectedMethod: "observe"})
				if len(cases) != 1 {
					t.Fatal("named observation differs")
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
				if !found || !strings.Contains(message, "REQUEST_BINDING_OBSERVATION") {
					t.Fatal("observation envelope differs: ", message)
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
	for _, key := range []string{"unrelated/anonymous", "unrelated/isTest", "qualified/anonymous", "qualified/isTest", "ownShadow/isTest"} {
		if !seen[key] {
			t.Fatal("missing native cell ", key)
		}
	}
}
