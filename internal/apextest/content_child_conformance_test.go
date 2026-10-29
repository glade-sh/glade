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
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type contentChildCell struct {
	Anonymous    string            `json:"anonymous"`
	Classes      map[string]string `json:"classes"`
	Tests        []string          `json:"tests"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
	Native       struct {
		Outcome          string         `json:"outcome"`
		Value            map[string]any `json:"value"`
		ExceptionMessage string         `json:"exceptionMessage"`
	} `json:"native"`
	Outcome       string `json:"outcome"`
	Primary       string `json:"primary"`
	ExceptionType string `json:"exceptionType"`
	Message       string `json:"message"`
}

func runContentChildConformance(t *testing.T, cluster string, shapes int) {
	t.Helper()
	var data struct {
		Cluster                string `json:"cluster"`
		ShapeCount             int    `json:"shapeCount"`
		NativeObservationCount int    `json:"nativeObservationCount"`
		Rows                   []struct {
			ID     string                                 `json:"id"`
			Routes map[string]map[string]contentChildCell `json:"routes"`
		} `json:"rows"`
	}
	var capture struct {
		APIVersions             []string                     `json:"apiVersions"`
		NativeObservationCount  int                          `json:"nativeObservationCount"`
		CarriedObservationCount int                          `json:"carriedObservationCount"`
		FixturesByAPI           map[string]map[string]string `json:"fixturesByAPI"`
	}
	read := func(name string, dst any) {
		raw, err := os.ReadFile(filepath.Join("testdata/conformance/content_child", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			t.Fatal(err)
		}
	}
	read(cluster, &data)
	read("capture-summary", &capture)
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != shapes*6 || capture.NativeObservationCount != 162 || capture.CarriedObservationCount != 0 || strings.Join(capture.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) != 2 {
			t.Fatal("missing or duplicate row/route")
		}
		seen[row.ID] = true
		for _, route := range []string{"anonymous", "istest"} {
			if len(row.Routes[route]) != 3 {
				t.Fatal("missing native API cells")
			}
		}
	}
	for _, api := range capture.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			fixtures := capture.FixturesByAPI[api]
			if len(fixtures) != 3 {
				t.Fatal("missing captured object metadata")
			}
			counts := newConformanceCountLog(t, "ContentChild-"+cluster, api)
			index := contentChildIndex(t, api, fixtures, nil)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "istest"} {
						t.Run(route, func(t *testing.T) {
							cell, ok := row.Routes[route][api]
							if !ok {
								t.Fatal("missing independent native cell")
							}
							sources := map[string]string{}
							if route == "anonymous" {
								sources["anonymous"] = cell.Anonymous
							}
							for name, source := range cell.Classes {
								sources["class:"+name] = source
							}
							if len(sources) == 0 || len(sources) != len(cell.SourceSHA256) {
								t.Fatal("incomplete source binding")
							}
							for name, source := range sources {
								sum := sha256.Sum256([]byte(source))
								if source == "" || hex.EncodeToString(sum[:]) != cell.SourceSHA256[name] {
									t.Fatal("captured source changed")
								}
							}
							kind := "exact"
							if cell.Outcome == "reject" {
								kind = "category"
							}
							counts.TrackKind(t, route, kind, 1)
							rowIndex := contentChildIndex(t, api, fixtures, cell.Classes)
							analysis := sema.Analyze(rowIndex)
							diagnostics := append(append([]diagnostic.Diagnostic{}, rowIndex.Diagnostics...), analysis.Diagnostics...)
							var execErr error
							if route == "anonymous" {
								// A rejection belongs to the captured anonymous unit, never its setup.
								contentChildCompile(t, contentChildCell{Outcome: "runtime"}, diagnostics, nil)
								analysis := sema.AnalyzeAnonymous(rowIndex, cell.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(cell.Anonymous, vm.CompileOptions{APIVersion: api})
								contentChildCompile(t, cell, analysis.Diagnostics, compileErr)
								if cell.Outcome == "reject" {
									return
								}
								runner := newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								contentChildCompile(t, cell, diagnostics, nil)
								if cell.Outcome == "reject" {
									return
								}
								if len(cell.Tests) != 1 || len(cell.Classes) != 1 {
									t.Fatal("one captured test unit required")
								}
								cases := Discover(rowIndex, Options{SelectedClasses: cell.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&rowIndex, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							contentChildValue(t, row.ID, cell, execErr)
						})
					}
				})
			}
		})
	}
}

func contentChildIndex(t *testing.T, api string, fixtures, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range fixtures {
		writeFile(t, filepath.Join(root, "force-app/main/default", name), source)
	}
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

// Native text stays in the fixture. Category-only assertions reject the same
// source unit without making Glade wording the Salesforce oracle.
func contentChildCompile(t *testing.T, want contentChildCell, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var messages []string
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			if want.Outcome == "reject" && len(want.Classes) != 0 {
				unit := strings.TrimSuffix(filepath.Base(d.File), ".cls")
				if _, captured := want.Classes[unit]; !captured {
					t.Fatalf("rejection outside captured source unit: %s: %s", d.File, d.Message)
				}
			}
			message := d.NativeMessage
			if message == "" {
				message = d.Message
			}
			if strings.TrimSpace(message) == "" {
				t.Fatal("empty Glade compile diagnostic")
			}
			messages = append(messages, message)
		}
	}
	if compileErr != nil && strings.TrimSpace(compileErr.Error()) != "" {
		messages = append(messages, compileErr.Error())
	}
	if want.Outcome == "reject" {
		if want.Native.Outcome != "compile-rejected" || strings.TrimSpace(want.Primary) == "" || len(messages) == 0 {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func contentChildExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func contentChildValue(t *testing.T, id string, want contentChildCell, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing observation exception: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		// The transport's type prefix is recorded intact; compare the raw Apex
		// message to ExceptionMessage, without local stack/report decorations.
		if want.Native.Outcome != "uncaught_anonymous" && want.Native.Outcome != "uncaught_test_failure" {
			t.Fatal("missing uncaught native answer")
		}
		if want.Native.ExceptionMessage != want.ExceptionType+": "+want.Message || contentChildExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s>; Glade <%s: %s>", want.Native.ExceptionMessage, runtimeErr.Type, message)
		}
		return
	}
	if want.Outcome != "runtime" || want.Native.Outcome != "runtime" || contentChildExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native marker payload; Glade <%v>", execErr)
	}
	marker := "C126_RESULT|" + id + "|"
	_, envelope, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "C126_RESULT|") != 1 {
		t.Fatalf("missing native envelope: %s", message)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(envelope), &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"caught", "exceptionType"} {
		if name, ok := got[key].(string); ok {
			got[key] = contentChildExceptionType(name)
		}
	}
	if !reflect.DeepEqual(got, want.Native.Value) {
		t.Fatalf("native payload <%v>; Glade <%s>", want.Native.Value, envelope)
	}
}

func TestContentChildLinksConformance(t *testing.T)    { runContentChildConformance(t, "links", 10) }
func TestContentChildSiblingConformance(t *testing.T)  { runContentChildConformance(t, "sibling", 1) }
func TestContentChildDescribeConformance(t *testing.T) { runContentChildConformance(t, "describe", 2) }
func TestContentChildNeighborsConformance(t *testing.T) {
	runContentChildConformance(t, "neighbors", 10)
}
func TestContentChildControlsConformance(t *testing.T) { runContentChildConformance(t, "controls", 4) }
