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

type flowCoreExpected struct {
	Outcome       string          `json:"outcome"`
	Value         json.RawMessage `json:"value"`
	Primary       string          `json:"primary"`
	Native        json.RawMessage `json:"native"`
	CarryReason   string          `json:"carryReason"`
	ExceptionType string          `json:"exceptionType"`
	Message       string          `json:"message"`
}

type flowCoreRoute struct {
	Anonymous    string            `json:"anonymous"`
	Classes      map[string]string `json:"classes"`
	Tests        []string          `json:"tests"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
	Expected     flowCoreExpected  `json:"expected"`
}

type flowCoreAPI struct {
	RejectedFlowFiles []string                 `json:"rejectedFlowFiles"`
	Files             map[string]string        `json:"files"`
	SourceSHA256      map[string]string        `json:"sourceSha256"`
	Routes            map[string]flowCoreRoute `json:"routes"`
}

type flowCoreRow struct {
	ID    string                 `json:"id"`
	ByAPI map[string]flowCoreAPI `json:"byAPI"`
}

// Flow deployment rejections compare the same metadata unit by category.
// Native diagnostic text is retained as evidence.
func runFlowCoreConformance(t *testing.T, cluster string, shapes, cells, carried int) {
	t.Helper()
	var data struct {
		APIVersions             []string      `json:"apiVersions"`
		Cluster                 string        `json:"cluster"`
		ShapeCount              int           `json:"shapeCount"`
		NativeObservationCount  int           `json:"nativeObservationCount"`
		CarriedObservationCount int           `json:"carriedObservationCount"`
		Rows                    []flowCoreRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/flow_core", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	apiVersions := strings.Join(data.APIVersions, ",")
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != cells || data.CarriedObservationCount != carried || (apiVersions != "62.0,67.0" && apiVersions != "62.0,67.0,68.0") {
		t.Fatal("incomplete admitted native matrix")
	}
	seen := map[string]bool{}
	observations, carries := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.ByAPI) != len(data.APIVersions) {
			t.Fatalf("missing/repeated native shape: %s", row.ID)
		}
		seen[row.ID] = true
		for _, api := range data.APIVersions {
			fixture, ok := row.ByAPI[api]
			if !ok || len(fixture.Routes) != 2 {
				t.Fatal("missing API or route")
			}
			flowCoreCheckHashes(t, fixture.Files, fixture.SourceSHA256)
			for _, route := range []string{"anonymous", "istest"} {
				source, ok := fixture.Routes[route]
				if !ok || !json.Valid(source.Expected.Native) {
					t.Fatal("missing native answer")
				}
				texts := source.Classes
				if route == "anonymous" {
					texts = map[string]string{"anonymous": source.Anonymous}
				}
				flowCoreCheckHashes(t, texts, source.SourceSHA256)
				switch source.Expected.Outcome {
				case "runtime":
					if !json.Valid(source.Expected.Value) {
						t.Fatal("missing native payload")
					}
				case "uncaught":
					if source.Expected.ExceptionType == "" || source.Expected.Message == "" {
						t.Fatal("empty native uncaught answer")
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
	bases := newConformanceSchemaCache(t)
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FlowCore-"+cluster, api)
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
							index := flowCoreIndex(t, api, source.Classes, fixture.Files)
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
									for _, path := range fixture.RejectedFlowFiles {
										rejected = rejected || d.Severity == diagnostic.Error && d.File == filepath.Join(p.Root, path)
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
							schemaFiles := conformanceFlowSchemaFiles(fixture.Files)
							base := bases.get(t, api, "", schemaFiles, func(t *testing.T) *conformanceRunner {
								return conformanceSchemaBase(t, flowCoreIndex(t, api, nil, schemaFiles), api)
							})
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: base.orgSource})
							machine := runner.newMachine()
							automation.ApplyToOrg(machine.Org, automations)
							// Flow rules differ per row. Detach metadata caches after applying them
							// to the row's private org; the linked passive runtime stays shared.
							machine.SetOrg(machine.Org)
							var execErr error
							if route == "anonymous" {
								analysis := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								if err != nil || analysis.HasErrors() {
									t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
								}
								_, execErr = machine.Execute(program)
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
								machine.EnableTestContext()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							flowCorePayload(t, row.ID, want, execErr)
						})
					}
				})
			}
		})
	}
}

func flowCoreCheckHashes(t *testing.T, sources, hashes map[string]string) {
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

func flowCoreIndex(t *testing.T, api string, classes, files map[string]string) typesys.Index {
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

func flowCoreExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func flowCorePayload(t *testing.T, id string, expected flowCoreExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if !errors.As(execErr, &runtimeErr) {
		t.Fatalf("missing captured marker assertion: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if expected.Outcome == "uncaught" {
		if flowCoreExceptionType(runtimeErr.Type) != expected.ExceptionType || message != expected.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", expected.ExceptionType, expected.Message, runtimeErr.Type, message)
		}
		return
	}
	if flowCoreExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("missing marker assertion: %v", execErr)
	}
	_, envelope, found := strings.Cut(message, "FLOW_CORE_RESULT|"+id+"|")
	if !found || strings.Count(message, "FLOW_CORE_RESULT|") != 1 {
		t.Fatalf("missing observation envelope: %s", message)
	}
	var got, want map[string]any
	if err := json.Unmarshal([]byte(envelope), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(expected.Value, &want); err != nil {
		t.Fatal(err)
	}
	if name, ok := got["exceptionType"].(string); ok {
		got["exceptionType"] = flowCoreExceptionType(name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native payload <%s>; Glade <%s>", expected.Value, envelope)
	}
}

func TestFlowCoreControlsConformance(t *testing.T) { runFlowCoreConformance(t, "controls", 3, 18, 0) }
func TestFlowCoreEngineControlConformance(t *testing.T) {
	runFlowCoreConformance(t, "engine-control", 1, 6, 0)
}
func TestFlowCoreOrderingConformance(t *testing.T) { runFlowCoreConformance(t, "ordering", 7, 42, 0) }
func TestFlowCoreLifecycleConformance(t *testing.T) {
	runFlowCoreConformance(t, "lifecycle", 13, 78, 0)
}
func TestFlowCoreChangedCriteriaConformance(t *testing.T) {
	runFlowCoreConformance(t, "changed-criteria", 16, 96, 10)
}
func TestFlowCoreFollowupsConformance(t *testing.T) {
	runFlowCoreConformance(t, "followups", 11, 44, 0)
}
func TestFlowCoreSubflowOutputsConformance(t *testing.T) {
	runFlowCoreConformance(t, "subflow-outputs", 6, 36, 2)
}
func TestFlowCoreDeleteFiltersConformance(t *testing.T) {
	runFlowCoreConformance(t, "delete-filters", 11, 66, 18)
}

func TestFlowCoreEntryConditionsConformance(t *testing.T) {
	runFlowCoreConformance(t, "entry-conditions", 33, 198, 0)
}
func TestFlowCoreAssignmentOperatorsConformance(t *testing.T) {
	runFlowCoreConformance(t, "assignment-operators", 35, 210, 10)
}
func TestFlowCoreNoMatchIdConformance(t *testing.T) {
	runFlowCoreConformance(t, "no-match-id", 1, 4, 0)
}
func TestFlowCoreBeforeSaveModesConformance(t *testing.T) {
	runFlowCoreConformance(t, "lifecycle-before-save", 1, 4, 0)
}
func TestFlowCoreEntryConditionNeighboursConformance(t *testing.T) {
	runFlowCoreConformance(t, "entry-condition-neighbours", 21, 84, 0)
}
