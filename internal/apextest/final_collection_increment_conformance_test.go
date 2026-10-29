package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type finalCollectionIncrementExpected struct {
	Outcome       string          `json:"outcome"`
	Value         string          `json:"value,omitempty"`
	Primary       string          `json:"primary,omitempty"`
	ExceptionType string          `json:"exceptionType,omitempty"`
	Message       string          `json:"message,omitempty"`
	Native        json.RawMessage `json:"native"`
}

type finalCollectionIncrementRoute struct {
	Anonymous     string                                      `json:"anonymous,omitempty"`
	Classes       map[string]string                           `json:"classes,omitempty"`
	Tests         []string                                    `json:"tests,omitempty"`
	SourceSHA256  map[string]string                           `json:"sourceSha256"`
	ExpectedByAPI map[string]finalCollectionIncrementExpected `json:"expectedByAPI,omitempty"`
	CarriedByAPI  map[string]json.RawMessage                  `json:"carriedByAPI,omitempty"`
}

type finalCollectionIncrementRow struct {
	ID          string                                   `json:"id"`
	Disposition string                                   `json:"disposition"`
	Routes      map[string]finalCollectionIncrementRoute `json:"routes"`
}

// Captured declarations and terminal assertions are unchanged. Compilation is
// checked before execution; a runtime ambiguity cannot satisfy a native compile
// rejection. The shared conformance runner clones the linked base per cell.
func runFinalCollectionIncrementConformance(t *testing.T, file, cluster string, shapes, cells, carried int) {
	t.Helper()
	var data struct {
		APIVersions             []string                      `json:"apiVersions"`
		Cluster                 string                        `json:"cluster"`
		ShapeCount              int                           `json:"shapeCount"`
		CarriedObservationCount int                           `json:"carriedObservationCount"`
		NativeObservationCount  int                           `json:"nativeObservationCount"`
		Rows                    []finalCollectionIncrementRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes ||
		data.NativeObservationCount != cells || data.CarriedObservationCount != carried || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatalf("native matrix: cluster=%s shapes=%d cells=%d APIs=%v", data.Cluster, len(data.Rows), data.NativeObservationCount, data.APIVersions)
	}
	seen := map[string]bool{}
	observations, carriedObservations := 0, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) == 0 {
			t.Fatalf("missing/repeated native row: %s", row.ID)
		}
		seen[row.ID] = true
		if row.Disposition != "conformance" && (row.Disposition != "infrastructure" || row.ID != "C166CONTROL_001") {
			t.Fatalf("unknown native disposition: %s/%s", row.ID, row.Disposition)
		}
		for route, source := range row.Routes {
			if route != "anonymous" && route != "isTest" {
				t.Fatalf("unknown native route: %s", route)
			}
			finalCollectionIncrementCheckSources(t, source)
			carriedObservations += len(source.CarriedByAPI)
			if row.Disposition == "infrastructure" {
				if len(source.ExpectedByAPI) != 0 || len(source.CarriedByAPI) != len(data.APIVersions) {
					t.Fatal("transport control must retain receipts without semantic expectations")
				}
				continue
			}
			if len(source.ExpectedByAPI)+len(source.CarriedByAPI) != len(data.APIVersions) {
				t.Fatalf("missing independent native cells: %s/%s", row.ID, route)
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
				if !ok {
					if receipt, carried := source.CarriedByAPI[api]; !carried || !json.Valid(receipt) {
						t.Fatal("missing carry receipt")
					}
					continue
				}
				if !ok || len(want.Native) == 0 {
					t.Fatalf("missing native answer: %s/%s/%s", row.ID, route, api)
				}
				switch want.Outcome {
				case "runtime":
					if !json.Valid([]byte(want.Value)) {
						t.Fatal("invalid native payload")
					}
				case "reject":
					if want.Primary == "" {
						t.Fatal("missing exact native diagnostic")
					}
				case "uncaught":
					if want.ExceptionType == "" || want.Message == "" {
						t.Fatal("missing exact native exception")
					}
				default:
					t.Fatalf("unsupported native outcome: %s", want.Outcome)
				}
				observations++
			}
		}
	}
	if observations != cells || carriedObservations != carried {
		t.Fatalf("independent semantic cells=%d, expected=%d", observations, cells)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "C166PF-"+cluster, api)
			baseIndex := finalCollectionIncrementIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				if row.Disposition == "infrastructure" {
					// This is capture transport, never a language match.
					continue
				}
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						source, captured := row.Routes[route]
						if !captured {
							// 037–040 have no anonymous static-class capture.
							continue
						}
						if _, answered := source.ExpectedByAPI[api]; !answered {
							t.Logf("carried %s/%s/%s: %s", row.ID, route, api, source.CarriedByAPI[api])
							continue
						}
						counts.Run(t, route, row.ID, route, func(t *testing.T) {
							want := source.ExpectedByAPI[api]
							var execErr error
							if route == "anonymous" {
								classes, body := finalCollectionIncrementAnonymous(t, source.Anonymous)
								index := sema.WithAnonymousDeclarationContext(finalCollectionIncrementIndex(t, api, classes))
								diagnostics := sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
								if !finalCollectionIncrementHasErrors(diagnostics) {
									diagnostics = sema.AnalyzeAnonymousDeclarationsInContext(index, index).Diagnostics
								}
								if finalCollectionIncrementHasErrors(diagnostics) {
									finalCollectionIncrementCompile(t, want, diagnostics, nil)
									return
								}
								analysis := sema.AnalyzeAnonymous(index, body, api)
								program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, body, api)})
								finalCollectionIncrementCompile(t, want, analysis.Diagnostics, compileErr)
								if want.Outcome == "reject" {
									return
								}
								runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
								if err != nil {
									t.Fatal(err)
								}
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: &org})
								_, execErr = runner.execute(program)
							} else {
								index := finalCollectionIncrementIndex(t, api, source.Classes)
								analysis := sema.Analyze(index)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								finalCollectionIncrementCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" {
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
							finalCollectionIncrementValue(t, row.ID, want, execErr)
						})
					}
				})
			}
		})
	}
}

func finalCollectionIncrementCheckSources(t *testing.T, source finalCollectionIncrementRoute) {
	t.Helper()
	values := map[string]string{}
	if source.Anonymous != "" {
		values["anonymous"] = source.Anonymous
	}
	for name, text := range source.Classes {
		values["class:"+name] = text
	}
	if len(values) == 0 || len(values) != len(source.SourceSHA256) {
		t.Fatal("captured source hash matrix incomplete")
	}
	for name, text := range values {
		sum := sha256.Sum256([]byte(text))
		if hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
			t.Fatalf("captured source changed: %s", name)
		}
	}
}

func finalCollectionIncrementIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	return loadTestIndex(t, root)
}

// Preserve original offsets while extracting the local declarations, following
// the anonymous CLI and AbstractOverride conformance request-runtime path.
func finalCollectionIncrementAnonymous(t *testing.T, source string) (map[string]string, string) {
	t.Helper()
	parsed := apexast.ParseSource("FinalCollectionIncrementAnonymous.cls", source)
	classes := map[string]string{}
	body := []byte(source)
	for _, decl := range parsed.Declarations {
		if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
			continue
		}
		start, end := decl.Range.Start.Offset, decl.Range.End.Offset
		if start < 0 || end <= start || end > len(source) {
			t.Fatal("invalid anonymous declaration source range")
		}
		padding := []byte(source[:start])
		for i := range padding {
			if padding[i] != '\n' && padding[i] != '\r' {
				padding[i] = ' '
			}
		}
		classes["GladeAnonymous"+decl.Name] = string(padding) + source[start:end]
		for i := start; i < end; i++ {
			if body[i] != '\n' && body[i] != '\r' {
				body[i] = ' '
			}
		}
	}
	return classes, string(body)
}

func finalCollectionIncrementHasErrors(diagnostics []diagnostic.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func finalCollectionIncrementCompile(t *testing.T, want finalCollectionIncrementExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
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
		if len(messages) == 0 || messages[0] != want.Primary {
			t.Fatalf("native compile rejection <%s>; Glade diagnostics=%v compile=%v", want.Primary, messages, compileErr)
		}
		return
	}
	if len(messages) != 0 || compileErr != nil {
		t.Fatalf("native compilation accepted; Glade diagnostics=%v compile=%v", messages, compileErr)
	}
}

func finalCollectionIncrementValue(t *testing.T, id string, want finalCollectionIncrementExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	if want.Outcome == "uncaught" {
		typeName := runtimeErr.Type
		// Qualify only the exception type; keep the native message exact.
		if !strings.Contains(typeName, ".") {
			typeName = "System." + typeName
		}
		message := runtimeErr.ExceptionMessage()
		if typeName != want.ExceptionType || message != want.Message {
			t.Fatalf("native <%s: %s>; Glade <%s: %s>", want.ExceptionType, want.Message, typeName, message)
		}
		return
	}
	if want.Outcome != "runtime" || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("native marker payload; Glade <%v>", execErr)
	}
	marker := "C126_RESULT|" + id + "|"
	message := runtimeErr.ExceptionMessage()
	_, observed, found := strings.Cut(message, marker)
	if !found || strings.Count(message, "C126_RESULT|") != 1 || observed != want.Value {
		t.Fatalf("native payload <%s>; Glade exception <%s>", want.Value, message)
	}
}

func TestFinalCollectionIncrementPfSeedsConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_PF_seeds.json", "PF-seeds", 28, 144, 24)
}
func TestFinalCollectionIncrementBindingSampleConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_binding_sample.json", "binding-sample", 24, 132, 0)
}
func TestFinalCollectionIncrementResultFormConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_result_form.json", "result-form", 8, 6, 42)
}
func TestFinalCollectionIncrementElementAssignmentConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_element_assignment.json", "element-assignment", 6, 36, 0)
}
func TestFinalCollectionIncrementExpressionConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_expression.json", "expression", 8, 24, 24)
}
func TestFinalCollectionIncrementFinalVariableRejectionConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_final_variable_rejection.json", "final-variable-rejection", 14, 84, 0)
}
func TestFinalCollectionIncrementA01ControlsConformance(t *testing.T) {
	// 095-097 carry the base-identical boxed Double JSON text mismatch
	// under; retain their sources and all six native receipts.
	runFinalCollectionIncrementConformance(t, "final_collection_increment_A01_controls.json", "A01-controls", 12, 54, 18)
}
func TestFinalCollectionIncrementLoopVariableConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_loop_variable.json", "loop-variable", 2, 12, 0)
}
func TestFinalCollectionIncrementBoundariesConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_boundaries.json", "boundaries", 12, 72, 0)
}
func TestFinalCollectionIncrementCatchVariableConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_catch_variable.json", "catch-variable", 6, 36, 0)
}
func TestFinalCollectionIncrementMustNotApplyConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_must_not_apply.json", "must-not-apply", 10, 60, 0)
}
func TestFinalCollectionIncrementTransportControlConformance(t *testing.T) {
	runFinalCollectionIncrementConformance(t, "final_collection_increment_transport_control.json", "transport-control", 1, 0, 6)
}
