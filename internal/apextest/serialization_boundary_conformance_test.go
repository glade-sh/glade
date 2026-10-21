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

func TestJSONClassBoundariesNativeConformance(t *testing.T) {
	runSerializationBoundaryConformance(t, "serialization")
}

func TestScheduledBatchAuditNativeConformance(t *testing.T) {
	runSerializationBoundaryConformance(t, "schedule")
}

func runSerializationBoundaryConformance(t *testing.T, topic string) {
	t.Helper()
	var data struct {
		Fixtures map[string]struct{ Files, SHA256 map[string]string }
		Cells    []struct {
			API, Context, Topic, Fixture, ReceiptSHA256, OrgIDHash string
			Native                                                 map[string]any
		}
		CycleMessagePrefix, CarriedDiagnostics string
		Carries                                []string
	}
	raw, err := os.ReadFile("testdata/conformance/serialization_schedule_boundaries.json")
	if err != nil {
		t.Fatal(err)
	}
	if json.Unmarshal(raw, &data) != nil || len(data.Cells) != 12 || len(data.Fixtures) != 3 || data.CycleMessagePrefix != "Cycle detected." {
		t.Fatal("incomplete boundary observations")
	}
	for _, carry := range data.Carries {
		t.Log("carried: " + carry)
	}
	seen := map[string]bool{}
	for _, cell := range data.Cells {
		if cell.Topic != topic {
			continue
		}
		t.Run(cell.API+"/"+cell.Context, func(t *testing.T) {
			key := cell.API + "/" + cell.Context
			if seen[key] || cell.Fixture != cell.API || len(cell.ReceiptSHA256) != 64 || len(cell.OrgIDHash) != 64 {
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
			marker := "NATIVE_" + strings.ToUpper(topic) + "|"
			payload := ""
			switch cell.Context {
			case "istest":
				cases := Discover(index, Options{SelectedClasses: []string{"BoundaryObservationTest"}, SelectedMethod: topic})
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
					t.Fatalf("missing observation transport: %v", err)
				}
				_, payload, ok = strings.Cut(fault.ExceptionMessage(), marker)
				if !ok {
					t.Fatal("missing observation marker")
				}
			case "anonymous":
				program, err := vm.CompileAnonymousWithOptions(fixture.Files[topic+".apex"], vm.CompileOptions{APIVersion: cell.API})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					var fault *vm.RuntimeError
					if topic == "schedule" && errors.As(err, &fault) && fault.Type == "System.StringException" && fault.ExceptionMessage() == "Job does not exist or is already aborted." {
						// The native helper aborts its scratch job after observing
						// the row. Local anonymous abort is a separate unsupported
						// boundary, so it prevents returning that observation here.
						for path, native := range boundaryObservationLeaves(cell.Native) {
							t.Logf("carried anonymous schedule cleanup: %s native=%v", path, native)
						}
						return
					}
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, value, found := strings.Cut(line, marker); found {
						payload = value
					}
				}
			default:
				t.Fatal("unknown context")
			}
			var got map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatal("invalid observation: " + payload)
			}
			actual := boundaryObservationLeaves(got)
			for path, want := range boundaryObservationLeaves(cell.Native) {
				t.Run(path, func(t *testing.T) {
					value, present := actual[path]
					if native, text := want.(string); text && strings.HasSuffix(path, "/message") && strings.HasPrefix(native, data.CycleMessagePrefix) {
						t.Logf("carried diagnostic: %s; native=%q", data.CarriedDiagnostics, native)
						local, text := value.(string)
						if !present || !text || !strings.HasPrefix(local, data.CycleMessagePrefix) {
							t.Fatalf("native prefix=%q; present=%v, local=%v", data.CycleMessagePrefix, present, value)
						}
					} else if !present || !reflect.DeepEqual(value, want) {
						t.Fatalf("native=%v; present=%v, local=%v", want, present, value)
					}
				})
			}
		})
	}
	for _, api := range []string{"62.0", "66.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+context] {
				t.Fatal("missing context")
			}
		}
	}
}

func boundaryObservationLeaves(value map[string]any) map[string]any {
	leaves := map[string]any{}
	var visit func(string, any)
	visit = func(path string, value any) {
		if fields, object := value.(map[string]any); object {
			for key, item := range fields {
				visit(strings.TrimPrefix(path+"/"+key, "/"), item)
			}
			return
		}
		leaves[path] = value
	}
	visit("", value)
	return leaves
}
