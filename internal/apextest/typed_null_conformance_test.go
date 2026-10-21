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

type typedNullExpected struct {
	Outcome            string          `json:"outcome"`
	Value              string          `json:"value,omitempty"`
	Primary            string          `json:"primary,omitempty"`
	ExceptionType      string          `json:"exceptionType,omitempty"`
	Message            string          `json:"message,omitempty"`
	Native             json.RawMessage `json:"native"`
	Marker             string          `json:"marker,omitempty"`
	StructuralJSON     bool            `json:"structuralJSON,omitempty"`
	FramingCarryReason string          `json:"framingCarryReason,omitempty"`
}

type typedNullSource struct {
	Anonymous    string            `json:"anonymous,omitempty"`
	Classes      map[string]string `json:"classes,omitempty"`
	Tests        []string          `json:"tests,omitempty"`
	SourceSHA256 map[string]string `json:"sourceSha256"`
	TestMethod   string            `json:"testMethod,omitempty"`
}

type typedNullRoute struct {
	typedNullSource
	SourcesByAPI        map[string]typedNullSource   `json:"sourcesByAPI,omitempty"`
	ExpectedByAPI       map[string]typedNullExpected `json:"expectedByAPI,omitempty"`
	LocalByAPI          map[string]typedNullExpected `json:"localByAPI,omitempty"`
	InfrastructureByAPI map[string]json.RawMessage   `json:"infrastructureByAPI,omitempty"`
}

type typedNullRow struct {
	ID          string                    `json:"id"`
	Disposition string                    `json:"disposition"`
	CarryReason string                    `json:"carryReason,omitempty"`
	Routes      map[string]typedNullRoute `json:"routes"`
}

// Captured declarations and terminal assertions are unchanged. Compilation is
// checked before execution; a runtime ambiguity cannot satisfy a native compile
// rejection. The shared conformance runner clones the linked base per cell.
func runTypedNullConformance(t *testing.T, file, cluster string, shapes, cells int) {
	t.Helper()
	var data struct {
		APIVersions            []string        `json:"apiVersions"`
		Cluster                string          `json:"cluster"`
		ShapeCount             int             `json:"shapeCount"`
		NativeObservationCount int             `json:"nativeObservationCount"`
		Rows                   []typedNullRow  `json:"rows"`
		ProjectSource          typedNullSource `json:"projectSource"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Cluster != cluster || data.ShapeCount != shapes || len(data.Rows) != shapes ||
		data.NativeObservationCount != cells || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatalf("native matrix: cluster=%s shapes=%d cells=%d APIs=%v", data.Cluster, len(data.Rows), data.NativeObservationCount, data.APIVersions)
	}
	if len(data.ProjectSource.Classes) != 0 {
		typedNullCheckSources(t, data.ProjectSource)
	}
	seen := map[string]bool{}
	observations := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) == 0 {
			t.Fatalf("missing/repeated native row: %s", row.ID)
		}
		seen[row.ID] = true
		if row.Disposition != "conformance" && row.Disposition != "carried" && (row.Disposition != "infrastructure" || row.ID != "C156TN_086") {
			t.Fatalf("unknown native disposition: %s/%s", row.ID, row.Disposition)
		}
		if row.Disposition == "carried" && row.CarryReason == "" {
			t.Fatal("carried native row needs a reason")
		}
		for route, source := range row.Routes {
			if route != "anonymous" && route != "isTest" {
				t.Fatalf("unknown native route: %s", route)
			}
			if len(source.SourcesByAPI) == 0 {
				typedNullCheckSources(t, source.typedNullSource)
			} else {
				if len(source.SourcesByAPI) != len(data.APIVersions) {
					t.Fatal("missing versioned native sources")
				}
				for _, api := range data.APIVersions {
					typedNullCheckSources(t, source.SourcesByAPI[api])
				}
			}
			if row.Disposition == "infrastructure" {
				if len(source.ExpectedByAPI) != 0 || len(source.InfrastructureByAPI) != len(data.APIVersions) {
					t.Fatal("transport control must retain receipts without semantic expectations")
				}
				continue
			}
			if len(source.ExpectedByAPI) != len(data.APIVersions) {
				t.Fatalf("missing independent native cells: %s/%s", row.ID, route)
			}
			if row.Disposition == "carried" && len(source.LocalByAPI) != len(data.APIVersions) {
				t.Fatal("carried row needs local observations at every API")
			}
			for _, api := range data.APIVersions {
				want, ok := source.ExpectedByAPI[api]
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
	if observations != cells {
		t.Fatalf("independent semantic cells=%d, expected=%d", observations, cells)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "C156TN-"+cluster, api)
			baseIndex := typedNullIndex(t, api, nil)
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			base := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{Org: &org})
			for _, row := range data.Rows {
				if row.Disposition == "infrastructure" {
					// This is capture transport, never an overload match or carry.
					continue
				}
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						source, captured := row.Routes[route]
						if !captured {
							// 038–043 have no anonymous static-class capture.
							continue
						}
						kind := "exact"
						if row.Disposition == "carried" {
							kind = "carried"
						}
						counts.RunKind(t, route, row.ID, route, kind, func(t *testing.T) {
							want := source.ExpectedByAPI[api]
							if want.FramingCarryReason != "" {
								counts.record(route, "line-framing", 1, true)
								t.Logf("compiler framing carried: %s; native=%s", want.FramingCarryReason, want.Native)
							}
							if row.Disposition == "carried" {
								t.Logf("carried: %s; native=%s; local=%s", row.CarryReason, want.Value, source.LocalByAPI[api].Value)
								want = source.LocalByAPI[api]
								if want.Outcome == "" {
									t.Fatal("missing carried local outcome")
								}
							}
							capturedSource := source.typedNullSource
							if len(source.SourcesByAPI) != 0 {
								capturedSource = source.SourcesByAPI[api]
							}
							projectClasses := make(map[string]string, len(data.ProjectSource.Classes)+len(capturedSource.Classes))
							for name, text := range data.ProjectSource.Classes {
								projectClasses[name] = text
							}
							for name, text := range capturedSource.Classes {
								if _, duplicate := projectClasses[name]; duplicate {
									t.Fatalf("duplicate captured class: %s", name)
								}
								projectClasses[name] = text
							}
							var execErr error
							if route == "anonymous" {
								classes, body := typedNullAnonymous(t, capturedSource.Anonymous)
								transient := sema.WithAnonymousDeclarationContext(typedNullIndex(t, api, classes))
								index := transient
								if len(projectClasses) != 0 {
									index = typedNullIndex(t, api, projectClasses)
									index.Types = append(index.Types, transient.Types...)
								}
								diagnostics := sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
								if !typedNullHasErrors(diagnostics) {
									diagnostics = sema.AnalyzeAnonymousDeclarationsInContext(index, transient).Diagnostics
								}
								if typedNullHasErrors(diagnostics) {
									typedNullCompile(t, want, diagnostics, nil)
									return
								}
								analysis := sema.AnalyzeAnonymous(index, body, api)
								program, compileErr := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
								typedNullCompile(t, want, analysis.Diagnostics, compileErr)
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
								index := typedNullIndex(t, api, projectClasses)
								analysis := sema.Analyze(index)
								diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
								typedNullCompile(t, want, diagnostics, nil)
								if want.Outcome == "reject" {
									return
								}
								if len(capturedSource.Tests) != 1 {
									t.Fatal("one captured test class required")
								}
								method := capturedSource.TestMethod
								if method == "" {
									method = "observe"
								}
								cases := Discover(index, Options{SelectedClasses: capturedSource.Tests, SelectedMethod: method})
								if len(cases) != 1 {
									t.Fatalf("named observation discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethods(cases)
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
							typedNullValue(t, row.ID, want, execErr)
						})
					}
				})
			}
		})
	}
}

func typedNullCheckSources(t *testing.T, source typedNullSource) {
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

func typedNullIndex(t *testing.T, api string, classes map[string]string) typesys.Index {
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
func typedNullAnonymous(t *testing.T, source string) (map[string]string, string) {
	t.Helper()
	parsed := apexast.ParseSource("TypedNullAnonymous.cls", source)
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

func typedNullHasErrors(diagnostics []diagnostic.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func typedNullCompile(t *testing.T, want typedNullExpected, diagnostics []diagnostic.Diagnostic, compileErr error) {
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

func typedNullValue(t *testing.T, id string, want typedNullExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("native %s; missing Glade exception: %v", want.Outcome, execErr)
	}
	if want.Outcome == "uncaught" {
		typeName := runtimeErr.Type
		// Match the System qualification used by HeapLimits' problemText,
		// restricted to the uncaught exception captured in this matrix.
		if typeName == "NullPointerException" {
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
	prefix := want.Marker
	if prefix == "" {
		prefix = "C156_RESULT|"
	}
	marker := prefix + id + "|"
	message := runtimeErr.ExceptionMessage()
	_, observed, found := strings.Cut(message, marker)
	equal := observed == want.Value
	if want.StructuralJSON {
		var actual, expected any
		equal = json.Unmarshal([]byte(observed), &actual) == nil && json.Unmarshal([]byte(want.Value), &expected) == nil && reflect.DeepEqual(actual, expected)
	}
	if !found || strings.Count(message, prefix) != 1 || !equal {
		t.Fatalf("native payload <%s>; Glade exception <%s>", want.Value, message)
	}
}
