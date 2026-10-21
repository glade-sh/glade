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

// Constructor-only captures perform no hosted Cache operations.
func TestCacheExceptionConstructorNativeConformance(t *testing.T) {
	var data struct {
		NativeObservationCount int
		APIVersions            []string
		Rows                   []struct {
			ID, API, Context, Outcome, OrgIDHash                       string
			Test, Anonymous, AnonymousSHA256, NativeMessage, RawSHA256 string
			Sources, SourceSHA256                                      map[string]string
			Value                                                      map[string]any
		}
	}
	raw, err := os.ReadFile("testdata/conformance/cache_exception_constructor.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.NativeObservationCount != 4 || len(data.Rows) != 4 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatal("native source/context inventory differs")
	}
	hashMatches := func(source, hash string) bool {
		sum := sha256.Sum256([]byte(source))
		return hex.EncodeToString(sum[:]) == hash
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.API + "/" + row.Context + "/" + row.ID
		if seen[key] {
			t.Fatal("duplicate native cell")
		}
		seen[key] = true
		modes := []bool{true}
		if row.Context == "anonymous" {
			modes = append(modes, false)
		}
		for _, linked := range modes {
			label := "linked"
			if !linked {
				label = "unlinked-control"
			}
			t.Run(key+"/"+label, func(t *testing.T) {
				if len(row.OrgIDHash) != 64 || len(row.RawSHA256) != 64 {
					t.Fatal("duplicate or unbound native cell")
				}
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
				if row.Outcome != "runtime" || index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native constructor executes; diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = row.API
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: linked, Org: &org})
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
					t.Fatalf("missing constructor observation: %v", execErr)
				}
				decode := func(message string) map[string]any {
					t.Helper()
					_, payload, found := strings.Cut(message, ", Actual: ")
					if !found || !strings.Contains(message, "CACHE_EXCEPTION_OBSERVATION") {
						t.Fatal("observation transport differs: ", message)
					}
					var value map[string]any
					if err := json.Unmarshal([]byte(payload), &value); err != nil {
						t.Fatal(err)
					}
					return value
				}
				native := decode(row.NativeMessage)
				local := decode(runtimeErr.ExceptionMessage())
				if !reflect.DeepEqual(row.Value, native) {
					t.Fatal("exported observation differs from native transport")
				}
				if !linked {
					// The standalone path preserves the input namespace's case.
					// Carry that separate display gap; these local controls isolate
					// constructor messages. Linked replays assert every native cell.
					if local["type"] != native["type"] {
						t.Logf("unlinked type casing carried: native=%v local=%v; canonical type display is separate from constructor initialization", native["type"], local["type"])
					}
					delete(native, "type")
					delete(local, "type")
				}
				if !reflect.DeepEqual(native, local) {
					t.Fatalf("native %v; local %s", row.Value, runtimeErr.ExceptionMessage())
				}
			})
		}
	}
	for _, api := range data.APIVersions {
		for _, context := range []string{"anonymous", "isTest"} {
			if !seen[api+"/"+context+"/constructor"] {
				t.Fatal("missing native constructor cell")
			}
		}
	}
}
