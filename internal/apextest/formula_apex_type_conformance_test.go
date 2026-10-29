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

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFormulaApexCoreOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "core", 21, 112)
}
func TestFormulaApexReturnsOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "returns", 12, 72)
}
func TestFormulaApexSObjectOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "sobject", 8, 64)
}
func TestFormulaApexPathsOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "paths", 7, 56)
}
func TestFormulaApexAccessOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "access", 10, 60)
}
func TestFormulaApexGettersOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "getters", 6, 40)
}
func TestFormulaApexRejectionsOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "rejections", 7, 56)
}
func TestFormulaApexGlobalOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "global", 26, 104)
}
func TestFormulaApexControlOrgConformance(t *testing.T) {
	runFormulaApexConformance(t, "control", 4, 32)
}

type formulaApexExpected struct {
	Outcome          string          `json:"outcome"`
	Value            json.RawMessage `json:"value"`
	ExceptionType    string          `json:"exceptionType"`
	ExceptionMessage string          `json:"exceptionMessage"`
	TestFailures     []struct {
		Class         string `json:"cls"`
		Method        string `json:"method"`
		ExceptionType string `json:"exceptionType"`
		Message       string `json:"message"`
	} `json:"testFailures"`
}

type formulaApexSource struct {
	ID                     string                         `json:"id"`
	Anonymous              string                         `json:"anonymous"`
	Classes                map[string]string              `json:"classes"`
	Tests                  []string                       `json:"tests"`
	SourceSHA256           map[string]string              `json:"sourceSha256"`
	ExpectedByAPI          map[string]formulaApexExpected `json:"expectedByAPI"`
	NativeDiagnosticsByAPI map[string][]string            `json:"nativeDiagnosticsByAPI"`
	CarryByAPI             map[string]string              `json:"carryByAPI"`
}

// Keep each captured route and declaration shape. Missing routes are excluded
// by the packet, never synthesized. API 59 is the same-release drift control.
// Each leaf uses the shared conformanceRunner and its own runtime clone.
func runFormulaApexConformance(t *testing.T, cluster string, shapes, cells int) {
	t.Helper()
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		ShapeCount             int      `json:"shapeCount"`
		NativeObservationCount int      `json:"nativeObservationCount"`
		Rows                   []struct {
			ID     string                       `json:"id"`
			Routes map[string]formulaApexSource `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance/formula_apex_type", cluster+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != shapes || data.ShapeCount != shapes || data.NativeObservationCount != cells || strings.Join(data.APIVersions, ",") != "59.0,62.0,67.0,68.0" {
		t.Fatal("incomplete captured formula matrix")
	}
	seen, observed := map[string]bool{}, 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) == 0 || len(row.Routes) > 2 {
			t.Fatalf("invalid/repeated formula row %s", row.ID)
		}
		seen[row.ID] = true
		for route, source := range row.Routes {
			if route != "anonymous" && route != "isTest" || source.ID != row.ID || len(source.ExpectedByAPI) != len(data.APIVersions) {
				t.Fatalf("invalid captured route %s/%s", row.ID, route)
			}
			formulaApexCheckSources(t, source)
			for _, api := range data.APIVersions {
				if _, ok := source.ExpectedByAPI[api]; !ok {
					t.Fatalf("missing captured cell %s/%s/%s", row.ID, route, api)
				}
				observed++
			}
		}
	}
	if observed != cells {
		t.Fatalf("captured cells %d; expected %d", observed, cells)
	}
	bases := newConformanceSchemaCache(t)
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "FormulaApex"+cluster, api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						source, captured := row.Routes[route]
						if !captured {
							continue
						}
						t.Run(route, func(t *testing.T) {
							if reason := source.CarryByAPI[api]; reason != "" {
								counts.TrackKind(t, route, "carried", 1)
								t.Logf("carried native cell %s/%s/%s: %s", row.ID, api, route, reason)
								return
							}
							counts.TrackKind(t, route, "exact", 1)
							want := source.ExpectedByAPI[api]
							if route == "anonymous" {
								formulaApexAnonymous(t, bases, api, source, want)
							} else {
								formulaApexTest(t, bases, api, source, want)
							}
						})
					}
				})
			}
		})
	}
}

func formulaApexCheckSources(t *testing.T, source formulaApexSource) {
	t.Helper()
	values := map[string]string{}
	if source.Anonymous != "" {
		values["anonymous"] = source.Anonymous
	}
	for name, text := range source.Classes {
		values["class:"+name] = text
	}
	if len(values) == 0 || len(values) != len(source.SourceSHA256) {
		t.Fatal("incomplete captured source hashes")
	}
	for key, text := range values {
		sum := sha256.Sum256([]byte(text))
		if hex.EncodeToString(sum[:]) != source.SourceSHA256[key] {
			t.Fatalf("captured source changed: %s/%s", source.ID, key)
		}
	}
}

func formulaApexIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
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

func formulaApexHasErrors(items []diagnostic.Diagnostic) bool {
	for _, item := range items {
		if item.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func formulaApexReject(t *testing.T, api string, source formulaApexSource, items []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	want := source.NativeDiagnosticsByAPI[api]
	if len(want) == 0 || want[0] == "" {
		t.Fatal("missing captured primary diagnostic")
	}
	actual := ""
	for _, item := range items {
		if item.Severity == diagnostic.Error {
			actual = item.NativeMessage
			if actual == "" {
				actual = item.Message
			}
			break
		}
	}
	if actual == "" && compileErr != nil {
		actual = compileErr.Error()
		var syntax *vm.ApexSyntaxError
		if errors.As(compileErr, &syntax) && syntax.NativeMessage != "" {
			actual = syntax.NativeMessage
		}
	}
	if actual != want[0] {
		t.Fatalf("native primary diagnostic <%s>; Glade <%s>; diagnostics=%v compile=%v", want[0], actual, items, compileErr)
	}
}

func formulaApexAnonymous(t *testing.T, bases *conformanceSchemaCache, api string, source formulaApexSource, want formulaApexExpected) {
	t.Helper()
	body := []byte(source.Anonymous)
	declarations := map[string]string{}
	parsed := apexast.ParseSource("FormulaApexAnonymous.cls", source.Anonymous)
	for _, decl := range parsed.Declarations {
		if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
			continue
		}
		start, end := decl.Range.Start.Offset, decl.Range.End.Offset
		if start < 0 || end <= start || end > len(body) {
			t.Fatal("invalid anonymous declaration range")
		}
		padding := []byte(source.Anonymous[:start])
		for i := range padding {
			if padding[i] != '\n' && padding[i] != '\r' {
				padding[i] = ' '
			}
		}
		declarations["GladeAnonymous"+decl.Name] = string(padding) + source.Anonymous[start:end]
		for i := start; i < end; i++ {
			if body[i] != '\n' && body[i] != '\r' {
				body[i] = ' '
			}
		}
	}
	index := sema.WithAnonymousDeclarationContext(formulaApexIndex(t, api, declarations))
	items := sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
	if !formulaApexHasErrors(items) {
		items = append(items, sema.AnalyzeAnonymousDeclarations(index).Diagnostics...)
	}
	if !formulaApexHasErrors(items) {
		items = append(items, sema.AnalyzeAnonymous(index, string(body), api).Diagnostics...)
	}
	program, compileErr := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, string(body), api)})
	if want.Outcome == "compile-rejected" {
		formulaApexReject(t, api, source, items, compileErr)
		return
	}
	if formulaApexHasErrors(items) || compileErr != nil {
		t.Fatalf("native executes; anonymous diagnostics=%v compile=%v", items, compileErr)
	}
	runtime := conformanceSchemaRequestRuntime(t, index)
	base := bases.get(t, api, "", nil, func(t *testing.T) *conformanceRunner {
		return conformanceSchemaBase(t, formulaApexIndex(t, api, nil), api)
	})
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: base.orgSource})
	_, execErr := runner.execute(program)
	formulaApexValue(t, source.ID, "anonymous", want, execErr)
}

func formulaApexTest(t *testing.T, bases *conformanceSchemaCache, api string, source formulaApexSource, want formulaApexExpected) {
	t.Helper()
	index := formulaApexIndex(t, api, source.Classes)
	analysis := sema.Analyze(index)
	items := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
	if want.Outcome == "compile-rejected" {
		formulaApexReject(t, api, source, items, nil)
		return
	}
	if formulaApexHasErrors(items) {
		t.Fatalf("native executes; named diagnostics=%v", items)
	}
	if len(source.Tests) != 1 {
		t.Fatal("one captured test class required")
	}
	cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: "observe"})
	if len(cases) != 1 {
		t.Fatalf("named observation discovery: %d", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	key := testCaseKey(cases[0])
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
	}
	base := bases.get(t, api, "", nil, func(t *testing.T) *conformanceRunner {
		return conformanceSchemaBase(t, formulaApexIndex(t, api, nil), api)
	})
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: base.orgSource})
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
	if want.Outcome == "rejected-with-tests" && (len(want.TestFailures) != 1 || want.TestFailures[0].Class != cases[0].ClassName || want.TestFailures[0].Method != "observe") {
		t.Fatal("uncaught answer must identify the captured test method")
	}
	formulaApexValue(t, source.ID, "isTest", want, execErr)
}

func formulaApexValue(t *testing.T, id, route string, want formulaApexExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native exception/transport missing: %v", execErr)
	}
	message := runtimeErr.ExceptionMessage()
	if want.Outcome == "runtime" {
		if runtimeErr.Type != "System.AssertException" {
			t.Fatalf("wrong native observation transport: %v", execErr)
		}
		marker := "C126_RESULT|" + id + "|"
		_, tail, found := strings.Cut(message, marker)
		if !found || strings.Count(message, "C126_RESULT|") != 1 {
			t.Fatalf("invalid observation marker: %s", message)
		}
		var observed, expected map[string]any
		if err := json.Unmarshal([]byte(tail), &observed); err != nil || len(observed) == 0 {
			t.Fatalf("invalid observation JSON: %s", tail)
		}
		if err := json.Unmarshal(want.Value, &expected); err != nil || len(expected) == 0 {
			t.Fatal("invalid captured value")
		}
		if !reflect.DeepEqual(expected, observed) {
			t.Fatalf("native value %s; Glade value %s", want.Value, tail)
		}
		return
	}
	if strings.Contains(message, "C126_RESULT|") {
		t.Fatal("uncaught native answer cannot become a transport marker")
	}
	nativeType, nativeMessage := want.ExceptionType, want.ExceptionMessage
	switch want.Outcome {
	case "uncaught_anonymous":
		if route != "anonymous" {
			t.Fatal("uncaught anonymous outcome on named route")
		}
	case "rejected-with-tests":
		if route != "isTest" || len(want.TestFailures) != 1 {
			t.Fatal("missing captured uncaught test failure")
		}
		nativeType, nativeMessage = want.TestFailures[0].ExceptionType, want.TestFailures[0].Message
	default:
		t.Fatalf("unsupported native observation %s", want.Outcome)
	}
	typeName := runtimeErr.Type
	// Match the typed-null adapter's System qualification, restricted to the
	// uncaught ListException captured here. ExceptionMessage stays exact.
	if typeName == "ListException" {
		typeName = "System." + typeName
	}
	if nativeType == "" || typeName != nativeType || (nativeMessage != message && nativeMessage != typeName+": "+message) {
		t.Fatalf("native exception <%s>; Glade <%s: %s>", nativeMessage, typeName, message)
	}
}
