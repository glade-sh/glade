package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// A List<SObject> view of one concrete record list converts implicitly into an
// unrelated record list. Native rows per API and route: assignment throws a
// TypeException, empty or not, stub-returned or plain; a method return of the
// same value succeeds; explicit casts throw. Custom object and custom metadata
// targets behave alike in both directions.
func TestImplicitListNarrowingOrgConformance(t *testing.T) {
	var data struct {
		APIVersions            []string            `json:"apiVersions"`
		Routes                 []string            `json:"routes"`
		ShapeCount             int                 `json:"shapeCount"`
		NativeObservationCount int                 `json:"nativeObservationCount"`
		MarkerPrefix           string              `json:"markerPrefix"`
		ObjectsByAPI           map[string][]string `json:"objectsByAPI"`
		NativeFiles            []struct {
			SHA256 string `json:"sha256"`
		} `json:"nativeFiles"`
		Rows []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				ByAPI map[string]stubDispatchSource `json:"byAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/implicit_list_narrowing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" ||
		data.ShapeCount != 18 || len(data.Rows) != data.ShapeCount || data.NativeObservationCount != 108 ||
		data.MarkerPrefix == "" || len(data.NativeFiles) != 24 {
		t.Fatal("incomplete native narrowing matrix")
	}
	seen := make(map[string]bool)
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("missing or repeated native narrowing shape")
		}
		seen[row.ID] = true
		for _, route := range data.Routes {
			if len(row.Routes[route].ByAPI) != len(data.APIVersions) {
				t.Fatal("missing independent native routes")
			}
			for _, api := range data.APIVersions {
				cell := row.Routes[route].ByAPI[api]
				texts := make(map[string]string)
				if cell.Anonymous != "" {
					texts["anonymous"] = cell.Anonymous
				}
				for name, text := range cell.Classes {
					texts["class:"+name] = text
				}
				stubDispatchCheckSources(t, texts, cell.SourceSHA256)
				if cell.Carry != "" {
					t.Fatal("unexpected carried narrowing row")
				}
				if row.ID == "N_C_N" || row.ID == "N_B_N" || row.ID == "N_C_NULL" || row.ID == "N_C_MIXED" {
					family := "implicit_list_assignment"
					if row.ID == "N_C_NULL" || row.ID == "N_C_MIXED" {
						family = "implicit_list_assignment_null"
					}
					path := filepath.Join("testdata/conformance", family, "native", "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv")
					native, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					records := strings.Split(strings.TrimSpace(string(native)), "\n")
					if len(records) != 3 || records[0] != "api\troute\tcase\tpayload" {
						t.Fatal("incomplete assignment/return observations")
					}
					found := false
					for _, line := range records[1:] {
						record := strings.Split(line, "\t")
						if len(record) != 4 || record[0] != api || record[1] != route {
							t.Fatal("invalid native assignment/return observation")
						}
						if record[2] == row.ID {
							var got, want any
							if json.Unmarshal([]byte(record[3]), &got) != nil || json.Unmarshal(cell.Expected.Value, &want) != nil || !reflect.DeepEqual(got, want) {
								t.Fatal("fresh native observation differs from matrix")
							}
							found = true
						}
					}
					if !found {
						t.Fatal("missing fresh native observation")
					}
				}
				if cell.Expected.Outcome != "runtime" || !json.Valid(cell.Expected.Value) ||
					(route == "anonymous") != (cell.Anonymous != "") {
					t.Fatal("missing native payload")
				}
			}
		}
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			objects := data.ObjectsByAPI[api]
			if len(objects) != 2 {
				t.Fatal("missing native fixture objects")
			}
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						t.Run(route, func(t *testing.T) {
							cell := row.Routes[route].ByAPI[api]
							index := implicitListNarrowingIndex(t, api, objects, cell.Classes)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), sema.Analyze(index).Diagnostics...)
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, cell.Anonymous, api)
								diagnostics = append(diagnostics, analysis.Diagnostics...)
								program, compileErr := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: api})
								stubDispatchCompile(t, cell.Expected, diagnostics, compileErr)
								_, execErr = runner.execute(program)
							} else {
								stubDispatchCompile(t, cell.Expected, diagnostics, nil)
								if len(cell.Tests) != 1 {
									t.Fatal("one native test entry required")
								}
								cases := Discover(index, Options{SelectedClasses: cell.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatal("native named entry not discovered")
								}
								methods, methodErrors := compileTestMethods(cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named entry compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							local, matches := implicitListNarrowingOutcome(data.MarkerPrefix+row.ID+"|", cell.Expected, execErr)
							switch {
							case cell.Carry == "" && !matches:
								t.Fatalf("native payload <%s>; local <%s>", cell.Expected.Value, local)
							case cell.Carry != "" && matches:
								t.Fatal("carried row now matches native; drop its carry")
							case cell.Carry != "":
								t.Logf("%s native <%s>; local <%s>", cell.Carry, cell.Expected.Value, local)
							}
						})
					}
				})
			}
		})
	}
}

// implicitListNarrowingOutcome reads the marker payload and reports whether
// it equals the native value after the shared normalization.
func implicitListNarrowingOutcome(marker string, want stubDispatchExpectation, execErr error) (string, bool) {
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || stubDispatchBuiltinExceptionName(runtimeErr.Type) != "System.AssertException" {
		return fmt.Sprint(execErr), false
	}
	_, text, ok := strings.Cut(runtimeErr.ExceptionMessage(), marker)
	if !ok {
		return runtimeErr.ExceptionMessage(), false
	}
	var actual, expected any
	if json.Unmarshal([]byte(text), &actual) != nil || json.Unmarshal(want.Value, &expected) != nil {
		return text, false
	}
	actual = stubDispatchNormalize(actual)
	payload, _ := json.Marshal(actual)
	return string(payload), reflect.DeepEqual(actual, expected)
}

func implicitListNarrowingIndex(t *testing.T, api string, objects []string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for _, object := range objects {
		label := strings.TrimSuffix(strings.TrimSuffix(object, "__c"), "__mdt")
		nameField := ""
		if strings.HasSuffix(object, "__c") {
			nameField = "<nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel>"
		}
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", object, object+".object-meta.xml"),
			`<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>`+label+`</label><pluralLabel>`+label+`</pluralLabel>`+nameField+`<visibility>Public</visibility></CustomObject>`)
	}
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}
