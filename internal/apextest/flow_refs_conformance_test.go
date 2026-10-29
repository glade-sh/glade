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

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type flowRefsExpected struct {
	Outcome     string          `json:"outcome"`
	Value       json.RawMessage `json:"value"`
	Primary     string          `json:"primary"`
	Native      json.RawMessage `json:"native"`
	CarryReason string          `json:"carryReason"`
}

type flowRefsRoute struct {
	Anonymous    string            `json:"anonymous"`
	Classes      map[string]string `json:"classes"`
	Tests        []string          `json:"tests"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
	Expected     flowRefsExpected  `json:"expected"`
}

type flowRefsAPI struct {
	Files        map[string]string        `json:"files"`
	SourceSHA256 map[string]string        `json:"sourceSha256"`
	Routes       map[string]flowRefsRoute `json:"routes"`
}

type flowRefsRow struct {
	ID    string                 `json:"id"`
	ByAPI map[string]flowRefsAPI `json:"byAPI"`
}

// Flow deployment rejections compare the same metadata unit by category;
// native diagnostic text is evidence, not Glade wording. Each fixture retains
// independent native observations for every API and route.
func runFlowRefsConformance(t *testing.T, cluster string, shapes, cells, carried int) {
	t.Helper()
	var data struct {
		APIVersions             []string      `json:"apiVersions"`
		Cluster                 string        `json:"cluster"`
		ShapeCount              int           `json:"shapeCount"`
		NativeObservationCount  int           `json:"nativeObservationCount"`
		CarriedObservationCount int           `json:"carriedObservationCount"`
		Rows                    []flowRefsRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/flow_refs", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != cells || data.CarriedObservationCount != carried || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatal("incomplete admitted native matrix")
	}
	seen := map[string]bool{}
	observations, carries := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.ByAPI) != 3 {
			t.Fatalf("missing/repeated native shape: %s", row.ID)
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			fixture, ok := row.ByAPI[api]
			if !ok || len(fixture.Routes) != 2 {
				t.Fatal("missing API or route")
			}
			flowRefsCheckHashes(t, fixture.Files, fixture.SourceSHA256)
			for _, route := range []string{"anonymous", "istest"} {
				source, ok := fixture.Routes[route]
				if !ok || !json.Valid(source.Expected.Native) {
					t.Fatal("missing native answer")
				}
				texts := source.Classes
				if route == "anonymous" {
					texts = map[string]string{"anonymous": source.Anonymous}
				}
				flowRefsCheckHashes(t, texts, source.SourceSHA256)
				switch source.Expected.Outcome {
				case "runtime":
					if !json.Valid(source.Expected.Value) {
						t.Fatal("missing native payload")
					}
				case "flow-deploy-rejected":
					if source.Expected.Primary == "" {
						t.Fatal("empty native rejection")
					}
				default:
					t.Fatalf("non-answer admitted: %s", source.Expected.Outcome)
				}
				if source.Expected.CarryReason != "" {
					carries++
				}
				observations++
			}
		}
	}
	if observations != cells || carries != carried {
		t.Fatalf("native cells/carries=%d/%d, want %d/%d", observations, carries, cells, carried)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FlowRefs-"+cluster, api)
			baseIndex := flowRefsIndex(t, api, nil, nil)
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					fixture := row.ByAPI[api]
					for _, route := range []string{"anonymous", "istest"} {
						t.Run(route, func(t *testing.T) {
							source := fixture.Routes[route]
							want := source.Expected
							if want.CarryReason != "" {
								counts.TrackKind(t, route, "carried", 1)
								t.Logf("carried %s/%s/%s: %s; native=%s", row.ID, api, route, want.CarryReason, want.Native)
								return
							}
							index := flowRefsIndex(t, api, source.Classes, fixture.Files)
							p, err := project.Load(index.Project.Root)
							if err != nil {
								t.Fatal(err)
							}
							automations, err := automation.LoadProject(p)
							if err != nil {
								t.Fatal(err)
							}
							if want.Outcome == "flow-deploy-rejected" {
								counts.TrackKind(t, route, "category", 1)
								rejected := false
								for _, d := range automations.Diagnostics {
									for _, path := range p.FlowFiles {
										rejected = rejected || d.Severity == diagnostic.Error && d.File == path
									}
								}
								if !rejected {
									t.Fatalf("native rejects Flow <%s>; Glade accepted the same metadata: %v", want.Primary, automations.Diagnostics)
								}
								return
							}
							counts.TrackKind(t, route, "exact", 1)
							if len(automations.Diagnostics) != 0 {
								t.Fatalf("native Flow accepted; Glade diagnostics=%v", automations.Diagnostics)
							}
							analysis := sema.Analyze(index)
							if index.HasErrors() || analysis.HasErrors() {
								t.Fatalf("native Apex accepted; Glade diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								if err != nil || analysis.HasErrors() {
									t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
								}
								_, execErr = runner.execute(program)
							} else {
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(source.Tests) != 1 || len(cases) != 1 {
									t.Fatal("one captured observation required")
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								machine := runner.newMachine()
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							flowRefsPayload(t, row.ID, want.Value, execErr)
						})
					}
				})
			}
		})
	}
}

func flowRefsCheckHashes(t *testing.T, sources, hashes map[string]string) {
	t.Helper()
	if len(sources) == 0 || len(sources) != len(hashes) {
		t.Fatal("incomplete source hashes")
	}
	for name, source := range sources {
		sum := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(sum[:]) != hashes[name] {
			t.Fatalf("captured source changed: %s", name)
		}
	}
}

func flowRefsIndex(t *testing.T, api string, classes, files map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for path, source := range files {
		writeFile(t, filepath.Join(root, path), source)
	}
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

func flowRefsExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func flowRefsPayload(t *testing.T, id string, expected json.RawMessage, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) || flowRefsExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing captured marker assertion: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	_, envelope, found := strings.Cut(message, "C126_RESULT|"+id+"|")
	if !found || strings.Count(message, "C126_RESULT|") != 1 {
		t.Fatalf("missing observation envelope: %s", message)
	}
	var got, want map[string]any
	if err := json.Unmarshal([]byte(envelope), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected, &want); err != nil {
		t.Fatal(err)
	}
	if name, ok := got["exceptionType"].(string); ok {
		got["exceptionType"] = flowRefsExceptionType(name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native payload <%s>; Glade <%s>", expected, envelope)
	}
}

func TestFlowRefsControlsConformance(t *testing.T) { runFlowRefsConformance(t, "controls", 2, 12, 0) }
func TestFlowRefsRecordConformance(t *testing.T)   { runFlowRefsConformance(t, "record", 23, 138, 12) }
func TestFlowRefsCollectionConformance(t *testing.T) {
	runFlowRefsConformance(t, "collection", 11, 66, 0)
}
func TestFlowRefsLookupConformance(t *testing.T) { runFlowRefsConformance(t, "lookup", 13, 78, 0) }

func TestFlowRefsSavedControlsConformance(t *testing.T) {
	runFlowRefsConformance(t, "saved_controls", 2, 12, 0)
}

func TestFlowRefsPriorRecordConformance(t *testing.T) {
	runFlowRefsConformance(t, "prior_record", 16, 96, 0)
}

func TestFlowRefsFirstRecordNullConformance(t *testing.T) {
	runFlowRefsConformance(t, "first_record_null", 11, 66, 0)
}

func TestFlowRefsRelationshipUpdatesConformance(t *testing.T) {
	runFlowRefsConformance(t, "relationship_update", 13, 78, 0)
}
