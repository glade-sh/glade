package apextest

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type recordCastViewsRoute struct {
	listSObjectCastRoute
	CallerAPIs       []string                       `json:"callerAPIs"`
	ClassAPIVersions map[string]string              `json:"classAPIVersions"`
	ExpectedByAPI    map[string]recordCastViewsCell `json:"expectedByAPI"`
}

type recordCastViewsCell struct {
	listSObjectCastExpected
	CallerAPI       string  `json:"callerAPI"`
	HelperAPI       *string `json:"helperAPI"`
	AssertionStatus string  `json:"assertionStatus"`
	CarryReason     string  `json:"carryReason,omitempty"`
}

type recordCastViewsHelper struct {
	Source       string `json:"source"`
	APIVersion   string `json:"apiVersion"`
	SourceSHA256 string `json:"sourceSha256"`
}

// Preserve the captured source and helper metadata independently of the
// caller's API. In particular, a modern caller must still execute helper54
// with that helper's historical source context.
func TestCollectionRecordCastViewsConformance(t *testing.T) {
	var data struct {
		APIVersions              []string                         `json:"apiVersions"`
		ShapeCount               int                              `json:"shapeCount"`
		NativeObservationCount   int                              `json:"nativeObservationCount"`
		AssertedObservationCount int                              `json:"assertedObservationCount"`
		CarriedObservationCount  int                              `json:"carriedObservationCount"`
		Marker                   string                           `json:"marker"`
		Objects                  map[string]string                `json:"objects"`
		ObjectSHA256             map[string]string                `json:"objectSha256"`
		Helpers                  map[string]recordCastViewsHelper `json:"helpers"`
		Rows                     []struct {
			ID     string                          `json:"id"`
			Routes map[string]recordCastViewsRoute `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/collection_record_cast_views.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "54.0,62.0,67.0,68.0" || data.ShapeCount != 87 || len(data.Rows) != 87 || data.NativeObservationCount != 462 || data.AssertedObservationCount != 454 || data.CarriedObservationCount != 8 || data.Marker == "" || len(data.Helpers) != 4 {
		t.Fatal("incomplete native matrix")
	}
	checkHash := func(name, source, want string) {
		t.Helper()
		sum := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(sum[:]) != want {
			t.Fatalf("captured source changed: %s", name)
		}
	}
	if len(data.Objects) != len(data.ObjectSHA256) {
		t.Fatal("incomplete object source hashes")
	}
	for name, source := range data.Objects {
		checkHash(name, source, data.ObjectSHA256[name])
	}
	for name, helper := range data.Helpers {
		checkHash(name, helper.Source, helper.SourceSHA256)
		if helper.APIVersion != strings.TrimPrefix(name, "CastViewHelper")+".0" {
			t.Fatalf("helper source API mismatch: %s", name)
		}
	}
	seen, cells, asserted, carried := map[string]bool{}, 0, 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("missing or repeated row/route")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "isTest"} {
			source, ok := row.Routes[route]
			if !ok || len(source.CallerAPIs) == 0 || len(source.ExpectedByAPI) != len(source.CallerAPIs) {
				t.Fatal("missing native cells")
			}
			texts := recordCastViewsClasses(t, source, data.Helpers)
			if route == "anonymous" {
				texts["anonymous"] = source.Anonymous
			}
			if len(texts) == 0 || len(texts) != len(source.SourceSHA256) {
				t.Fatal("incomplete source hashes")
			}
			for name, text := range texts {
				checkHash(name, text, source.SourceSHA256[name])
			}
			for _, api := range source.CallerAPIs {
				cell, ok := source.ExpectedByAPI[api]
				want := cell.listSObjectCastExpected
				if !ok || !json.Valid(want.Native) || cell.CallerAPI != api {
					t.Fatal("missing native answer")
				}
				if cell.HelperAPI != nil {
					name := "CastViewHelper" + strings.TrimSuffix(*cell.HelperAPI, ".0")
					if source.ClassAPIVersions[name] != *cell.HelperAPI {
						t.Fatal("helper metadata differs from native cell")
					}
				}
				switch cell.AssertionStatus {
				case "ASSERTED":
					if cell.CarryReason != "" {
						t.Fatal("asserted cell has carry reason")
					}
					asserted++
				case "CARRIED":
					if row.ID != "CTRL_POS" || cell.CarryReason == "" {
						t.Fatal("unexpected or unexplained carry")
					}
					carried++
				default:
					t.Fatal("missing native assertion status")
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if want.Primary == "" {
						t.Fatal("empty native compile rejection")
					}
				case "uncaught":
					if want.ExceptionType == "" || want.Message == "" {
						t.Fatal("empty native exception")
					}
				default:
					t.Fatal("non-answer admitted")
				}
				cells++
			}
		}
	}
	if cells != data.NativeObservationCount || asserted != data.AssertedObservationCount || carried != data.CarriedObservationCount {
		t.Fatal("native cell count differs")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "CollectionRecordCastViews", api)
			baseIndex := recordCastViewsIndex(t, api, recordCastViewsRoute{}, data.Helpers, data.Objects)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						source := row.Routes[route]
						cell, captured := source.ExpectedByAPI[api]
						if !captured {
							continue
						}
						kind := "exact"
						if cell.AssertionStatus == "CARRIED" {
							kind = "carried"
						}
						counts.RunKind(t, route, row.ID, route, kind, func(t *testing.T) {
							if kind == "carried" {
								t.Logf("retained native answer; carried: %s", cell.CarryReason)
								return
							}
							want := cell.listSObjectCastExpected
							index := recordCastViewsIndex(t, api, source, data.Helpers, data.Objects)
							rowOrg := org.Clone()
							if route == "anonymous" && strings.HasPrefix(row.ID, "ASSIGN_") {
								// Native anonymous assignment controls observed one
								// ambient Account; their owned Id filter excludes it.
								account := rowOrg.Objects["Account"]
								if len(account.Records) != 0 {
									t.Fatal("assignment fixture already has ambient records")
								}
								id := storage.ID("001000000000099")
								account.Records = map[storage.ID]storage.Record{id: {ID: id, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Ambient assignment control")}}}
								rowOrg.Objects["Account"] = account
							}
							analysis := sema.Analyze(index)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
							var execErr error
							if route == "anonymous" {
								anonymous := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								diagnostics = append(diagnostics, anonymous.Diagnostics...)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								listSObjectCastCompile(t, want, diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
								if err != nil {
									t.Fatal(err)
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: &rowOrg})
								_, execErr = runner.execute(program)
							} else {
								listSObjectCastCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" {
									return
								}
								if len(source.Tests) != 1 {
									t.Fatal("one observation class required")
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &rowOrg})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							recordCastViewsValue(t, data.Marker+row.ID+"|", want, execErr)
						})
					}
				})
			}
		})
	}
}

func recordCastViewsValue(t *testing.T, marker string, want listSObjectCastExpected, execErr error) {
	t.Helper()
	if want.Outcome == "uncaught" {
		collectionCastReflectionValue(t, marker, want, execErr)
		return
	}
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || listSObjectCastExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing observation exception: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, payload, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 {
		t.Fatalf("missing payload: %s", message)
	}
	var got, expected map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want.Value), &expected); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"exceptionType", "castType", "reflectionType"} {
		if name, ok := got[key].(string); ok {
			got[key] = listSObjectCastExceptionType(name)
		}
	}
	if expected["returnedId"] == "[ID]" {
		// Native evidence persistence redacts record Ids. Keep the source's
		// returnedOwned/returnedNull assertions, validate the local Id shape,
		// and apply only that same field's persistence projection.
		id, ok := got["returnedId"].(string)
		if !ok || storage.ValidateID(storage.ID(id)) != nil {
			t.Fatalf("invalid returned record Id: %v", got["returnedId"])
		}
		got["returnedId"] = "[ID]"
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native <%s>; Glade <%s>", want.Value, payload)
	}
}

func recordCastViewsClasses(t *testing.T, source recordCastViewsRoute, helpers map[string]recordCastViewsHelper) map[string]string {
	t.Helper()
	texts := make(map[string]string, len(source.ClassAPIVersions))
	for name, api := range source.ClassAPIVersions {
		text, ok := source.Classes[name]
		if !ok {
			helper, found := helpers[name]
			if !found || api != helper.APIVersion {
				t.Fatalf("missing helper or source API: %s", name)
			}
			text = helper.Source
		} else if api != "caller" {
			t.Fatalf("observation class source API differs: %s", name)
		}
		texts["class:"+name] = text
	}
	for name := range source.Classes {
		if _, ok := source.ClassAPIVersions[name]; !ok {
			t.Fatalf("missing class source API: %s", name)
		}
	}
	return texts
}

func recordCastViewsIndex(t *testing.T, api string, source recordCastViewsRoute, helpers map[string]recordCastViewsHelper, objects map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for key, text := range recordCastViewsClasses(t, source, helpers) {
		name := strings.TrimPrefix(key, "class:")
		classAPI := source.ClassAPIVersions[name]
		if classAPI == "caller" {
			classAPI = api
		}
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, text)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+classAPI+"</apiVersion></ApexClass>")
	}
	for name, text := range objects {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml"), text)
	}
	return loadTestIndex(t, root)
}
