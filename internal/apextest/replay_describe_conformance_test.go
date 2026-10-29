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

type replayDescribeExpected struct {
	Outcome             string          `json:"outcome"`
	Value               string          `json:"value,omitempty"`
	Primary             string          `json:"primary,omitempty"`
	ExceptionType       string          `json:"exceptionType,omitempty"`
	Message             string          `json:"message,omitempty"`
	Native              json.RawMessage `json:"native"`
	GladeDisposition    string          `json:"gladeDisposition,omitempty"`
	EventIDNullnessOnly bool            `json:"eventIDNullnessOnly,omitempty"`
	CarryReason         string          `json:"carryReason,omitempty"`
}
type replayDescribeRoute struct {
	OriginalApex       string                            `json:"originalApex"`
	OriginalApexSHA256 string                            `json:"originalApexSha256"`
	Anonymous          string                            `json:"anonymous,omitempty"`
	Classes            map[string]string                 `json:"classes,omitempty"`
	Tests              []string                          `json:"tests,omitempty"`
	SourceSHA256       map[string]string                 `json:"sourceSha256"`
	ExpectedByAPI      map[string]replayDescribeExpected `json:"expectedByAPI"`
}
type replayDescribeRow struct {
	ID     string                         `json:"id"`
	Routes map[string]replayDescribeRoute `json:"routes"`
}

// Keep captured operations and normalization; adapt only the marker transport. Each route/API
// checks compilation before executing on a private shared-runner clone.
func runReplayDescribeConformance(t *testing.T, cluster string, shapes, cells, carried int, apiVersions ...string) {
	t.Helper()
	if len(apiVersions) == 0 {
		apiVersions = []string{"59.0", "62.0", "67.0", "68.0"}
	}
	var data struct {
		APIVersions              []string            `json:"apiVersions"`
		Cluster                  string              `json:"cluster"`
		ShapeCount               int                 `json:"shapeCount"`
		NativeObservationCount   int                 `json:"nativeObservationCount"`
		AssertedObservationCount int                 `json:"assertedObservationCount"`
		CarriedObservationCount  int                 `json:"carriedObservationCount"`
		Rows                     []replayDescribeRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/replay_describe", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes || data.NativeObservationCount != cells || data.AssertedObservationCount != cells-carried || data.CarriedObservationCount != carried || strings.Join(data.APIVersions, ",") != strings.Join(apiVersions, ",") {
		t.Fatal("incomplete native matrix")
	}
	seen := map[string]bool{}
	observations := 0
	carriedObservations := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) == 0 {
			t.Fatalf("missing/repeated row or route: %s", row.ID)
		}
		seen[row.ID] = true
		if len(row.Routes) != 2 || len(row.Routes["anonymous"].ExpectedByAPI) != len(apiVersions) || len(row.Routes["isTest"].ExpectedByAPI) != len(apiVersions) {
			t.Fatal("missing anonymous or @IsTest route")
		}
		for _, route := range []string{"anonymous", "isTest", "named"} {
			source, ok := row.Routes[route]
			if !ok {
				continue
			}
			if len(source.ExpectedByAPI) != len(apiVersions) {
				t.Fatal("missing independent native cells")
			}
			originalSum := sha256.Sum256([]byte(source.OriginalApex))
			if source.OriginalApex == "" || hex.EncodeToString(originalSum[:]) != source.OriginalApexSHA256 {
				t.Fatal("captured operation changed")
			}
			values := map[string]string{}
			if route == "anonymous" {
				values["anonymous"] = source.Anonymous
			}
			for name, text := range source.Classes {
				values["class:"+name] = text
			}
			if len(values) == 0 || len(values) != len(source.SourceSHA256) {
				t.Fatal("incomplete source hash matrix")
			}
			for name, text := range values {
				sum := sha256.Sum256([]byte(text))
				if text == "" || hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
					t.Fatalf("captured source changed: %s", name)
				}
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok || !json.Valid(want.Native) {
					t.Fatal("missing native receipt")
				}
				if want.GladeDisposition != "" || want.CarryReason != "" {
					t.Fatal("unexpected carry: all admitted cells have native answers")
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if want.Primary == "" {
						t.Fatal("empty native compile diagnostic")
					}
				case "accept":
					if route != "named" {
						t.Fatal("compile acceptance requires named route")
					}
				case "uncaught":
					if want.ExceptionType == "" || want.Message == "" {
						t.Fatal("empty native exception")
					}
				default:
					t.Fatalf("non-answer admitted: %s", want.Outcome)
				}
				observations++
			}
		}
	}
	if observations != cells || carriedObservations != carried {
		t.Fatalf("native cells=%d, carried=%d, want=%d/%d", observations, carriedObservations, cells, carried)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			// Each runtime cell executes on a private clone of the API base.
			counts := newConformanceCountLog(t, "ReplayDescribe-"+cluster, api)
			baseIndex := replayDescribeIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			storage.EnsureStandardObject(&org, "BatchApexErrorEvent")
			storage.EnsureUniqueKeyPrefixes(&org)
			prefixes := map[string]string{"e00": org.Objects["C168A"+api[:2]+"00000000Event__e"].Definition.KeyPrefix, "1QQ": org.Objects["BatchApexErrorEvent"].Definition.KeyPrefix}
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org, LinkProject: true})
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest", "named"} {
						source, captured := row.Routes[route]
						if !captured {
							continue
						}
						t.Run(route, func(t *testing.T) {
							want := source.ExpectedByAPI[api]
							if want.CarryReason != "" {
								counts.TrackKind(t, route, "carried", 1)
								t.Logf("carried %s/%s/%s: %s; native rejection <%s>", row.ID, route, api, want.CarryReason, want.Primary)
								return
							}
							kind := "exact"
							if source.ExpectedByAPI[api].Outcome == "reject" {
								kind = "category"
							}
							counts.TrackKind(t, route, kind, 1)
							var execErr error
							if route == "anonymous" {
								index := replayDescribeIndex(t, api, source.Classes)
								setup := sema.Analyze(index)
								if index.HasErrors() || setup.HasErrors() {
									t.Fatalf("native setup accepted; Glade diagnostics=%v %v", index.Diagnostics, setup.Diagnostics)
								}
								analysis := sema.AnalyzeAnonymous(index, replayDescribeBind(source.Anonymous, api), api)
								program, compileErr := vm.CompileAnonymousWithOptions(replayDescribeBind(source.Anonymous, api), vm.CompileOptions{APIVersion: api})
								replayDescribeCompile(t, want, analysis.Diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								index := replayDescribeIndex(t, api, source.Classes)
								analysis := sema.Analyze(index)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								if want.Outcome == "reject" {
									if len(source.Tests) != 1 {
										t.Fatal("one rejected source unit required")
									}
									rejectedUnit := false
									for _, d := range diagnostics {
										if d.Severity == diagnostic.Error && filepath.Base(d.File) == source.Tests[0]+".cls" {
											rejectedUnit = true
										}
									}
									if !rejectedUnit {
										t.Fatalf("native rejected observation unit; Glade diagnostics=%v", diagnostics)
									}
								}
								replayDescribeCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" || want.Outcome == "accept" {
									return
								}
								if len(source.Tests) != 1 {
									t.Fatal("one captured test class required")
								}
								cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
								if len(cases) != 1 {
									t.Fatalf("named observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethodsForIndex(&index, cases)
								programs, programErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if methodErrors[key] != nil || programErrors[key] != nil {
									t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
								machine := runner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								_, execErr = machine.ExecuteInClass(programs[key], cases[0].ClassName)
							}
							replayDescribeValue(t, row.ID, want, execErr, prefixes)
						})
					}
				})
			}
		})
	}
}

func replayDescribeIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	fixtureRoot := "testdata/conformance/replay_describe"
	helper, err := os.ReadFile(filepath.Join(fixtureRoot, "CaptureValues.cls"))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(helper)
	if hex.EncodeToString(sum[:]) != "1a9b2f2e02583d449d1cba0cd6299844922c69abe63b1aca002ed261ad39a3ec" {
		t.Fatal("captured normalizer changed")
	}
	helperSource := string(helper)
	if len(classes) != 0 {
		helperSource = "@IsTest " + helperSource
	}
	allClasses := map[string]string{"C168CaptureValues": helperSource}
	for name, source := range classes {
		allClasses[name] = replayDescribeBind(source, api)
	}
	for _, item := range []struct{ stem, suffix string }{{"Event", "__e"}, {"Record", "__c"}} {
		metadata, err := os.ReadFile(filepath.Join(fixtureRoot, item.stem+".object"))
		if err != nil {
			t.Fatal(err)
		}
		name := "C168A" + api[:2] + "00000000" + item.stem + item.suffix
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", name+".object"), string(metadata))
	}
	for name, source := range allClasses {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

// Compile rejection cells retain the exact native diagnostic as evidence.
// Glade must reject the same source unit; its diagnostic wording is independent.
func replayDescribeCompile(t *testing.T, want replayDescribeExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	var messages []string
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			message := d.NativeMessage
			if message == "" {
				message = d.Message
			}
			messages = append(messages, message)
		}
	}
	if len(messages) == 0 && compileErr != nil {
		message := compileErr.Error()
		var syntax *vm.ApexSyntaxError
		if errors.As(compileErr, &syntax) && syntax.NativeMessage != "" {
			message = syntax.NativeMessage
		}
		messages = append(messages, message)
	}
	if want.Outcome == "reject" {
		if want.Primary == "" || len(messages) == 0 {
			t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func replayDescribeExceptionType(name string) string {
	if !strings.Contains(name, ".") {
		return "System." + name
	}
	return name
}

func replayDescribeValue(t *testing.T, id string, want replayDescribeExpected, execErr error, prefixes map[string]string) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	// ExceptionMessage is the raw Apex message, unlike Error/report decoration.
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "uncaught" {
		if replayDescribeExceptionType(runtimeErr.Type) != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, runtimeErr.Type, message)
		}
		return
	}
	if want.Outcome != "runtime" || replayDescribeExceptionType(runtimeErr.Type) != "System.AssertException" {
		t.Fatalf("native marker payload; Glade <%v>", execErr)
	}
	marker := "E15_RESULT|" + id + "|"
	_, envelope, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "E15_RESULT|") != 1 {
		t.Fatalf("missing native envelope: %s", message)
	}
	payload := []byte(envelope)
	var got, expected map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want.Value), &expected); err != nil {
		t.Fatal(err)
	}
	// Qualify only exception type fields, including nested attempts.
	replayDescribeQualify(got)
	replayDescribeBindIDPrefixes(expected, prefixes)
	if want.EventIDNullnessOnly {
		if observation, ok := got["value"].(map[string]any); ok {
			if identity, ok := observation["id"].(map[string]any); ok {
				if _, allocated := identity["idPrefix"]; allocated {
					observation["id"] = map[string]any{"isNull": false}
				}
			}
		}
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("native payload <%s>; Glade <%s>", want.Value, payload)
	}
}

func replayDescribeBind(source, api string) string {
	token := "C168A" + api[:2] + "00000000"
	return strings.NewReplacer("RpEvent__e", token+"Event__e", "RpRecord__c", token+"Record__c").Replace(source)
}
func replayDescribeQualify(value any) {
	switch x := value.(type) {
	case map[string]any:
		for key, child := range x {
			if key == "exceptionType" {
				if name, ok := child.(string); ok {
					x[key] = replayDescribeExceptionType(name)
				}
			} else {
				replayDescribeQualify(child)
			}
		}
	case []any:
		for _, child := range x {
			replayDescribeQualify(child)
		}
	}
}
func TestReplayDescribeTransportControlConformance(t *testing.T) {
	runReplayDescribeConformance(t, "transport_control", 1, 8, 0)
}
func TestReplayDescribeDescribePutConformance(t *testing.T) {
	runReplayDescribeConformance(t, "describe_put", 41, 328, 0)
}
func TestReplayDescribeConstructorsConformance(t *testing.T) {
	runReplayDescribeConformance(t, "constructors", 13, 104, 0)
}
func TestReplayDescribeDMLPublishConformance(t *testing.T) {
	runReplayDescribeConformance(t, "dml_publish", 18, 144, 0)
}
func TestReplayDescribeOrdinaryControlsConformance(t *testing.T) {
	runReplayDescribeConformance(t, "ordinary_controls", 5, 40, 0)
}
func TestReplayDescribeStandardDescribeConformance(t *testing.T) {
	runReplayDescribeConformance(t, "standard_describe", 4, 32, 0)
}
func TestReplayDescribeStandardGetConformance(t *testing.T) {
	runReplayDescribeConformance(t, "standard_get", 4, 32, 0)
}
func TestReplayDescribeStandardPutConformance(t *testing.T) {
	runReplayDescribeConformance(t, "standard_put", 8, 64, 0)
}
func TestReplayDescribeStandardConstructorConformance(t *testing.T) {
	runReplayDescribeConformance(t, "standard_constructor", 4, 32, 0)
}
func TestReplayDescribeStandardDMLPublishConformance(t *testing.T) {
	runReplayDescribeConformance(t, "standard_dml_publish", 2, 16, 0)
}

// Native event key prefixes are allocations in the captured org. Compare the
// returned prefix to this fixture's own schema allocation, retaining exact
// native ID length, nullness, UUID shape and message observations.
func replayDescribeBindIDPrefixes(value any, prefixes map[string]string) {
	switch x := value.(type) {
	case map[string]any:
		for key, child := range x {
			if key == "idPrefix" {
				if prefix, ok := child.(string); ok {
					if bound, exists := prefixes[prefix]; exists {
						x[key] = bound
					}
				}
			} else {
				replayDescribeBindIDPrefixes(child, prefixes)
			}
		}
	case []any:
		for _, child := range x {
			replayDescribeBindIDPrefixes(child, prefixes)
		}
	}
}

func TestPlatformEventSourceVersionConformance(t *testing.T) {
	runReplayDescribeConformance(t, "event_source_version", 3, 24, 0, "63.0", "64.0", "65.0", "66.0")
}
