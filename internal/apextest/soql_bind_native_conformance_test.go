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

// Keep the captured Apex intact. Compilation follows the earlier native
// exporters; execution and transaction isolation use conformanceRunner.
// Source bytes vary by API for fresh User seeds and retain their request hashes.
type soqlBindNativeDiagnostic struct {
	Problem string `json:"problem"`
}

type soqlBindNativeRoute struct {
	ID                     string                                `json:"id"`
	Anonymous              string                                `json:"anonymous,omitempty"`
	Classes                map[string]string                     `json:"classes,omitempty"`
	Objects                map[string]string                     `json:"objects,omitempty"`
	SetupClasses           map[string]string                     `json:"setupClasses,omitempty"`
	SetupObjects           map[string]string                     `json:"setupObjects,omitempty"`
	Tests                  []string                              `json:"tests,omitempty"`
	Stage                  string                                `json:"stage"`
	NativeSetupRejection   string                                `json:"nativeSetupRejection,omitempty"`
	NativeDiagnostics      []soqlBindNativeDiagnostic            `json:"nativeDiagnostics,omitempty"`
	NativeDiagnosticsByAPI map[string][]soqlBindNativeDiagnostic `json:"nativeDiagnosticsByAPI,omitempty"`
	SourceSHA256           map[string]string                     `json:"sourceSha256"`
}

type soqlBindNativeExpected struct {
	Outcome string          `json:"outcome"`
	Value   json.RawMessage `json:"value"`
}

type soqlBindNativeSource struct {
	soqlBindNativeRoute
}

type soqlBindNativeCell struct {
	Source   soqlBindNativeSource            `json:"source"`
	Expected *soqlBindNativeExpected         `json:"expected"`
	Carry    *struct{ Owner, Reason string } `json:"carry"`
}

func runSOQLBindNativeConformance(t *testing.T, file, family string, shapes, cells int) {
	t.Helper()
	var data struct {
		APIVersions       []string `json:"apiVersions"`
		Routes            []string `json:"routes"`
		ShapeCount        int      `json:"shapeCount"`
		NativeCellCount   int      `json:"nativeCellCount"`
		AssertedCellCount int      `json:"assertedCellCount"`
		ResolvedCellCount int      `json:"resolvedCellCount"`
		Rows              []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				Cells map[string]soqlBindNativeCell `json:"cells"`
			} `json:"routes"`
			RouteBoundaries map[string]string `json:"routeBoundaries"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != shapes || data.ShapeCount != shapes || data.NativeCellCount != cells || strings.Join(data.APIVersions, ",") != "59.0,62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatalf("incomplete native matrix: %s", file)
	}
	seen := map[string]bool{}
	actualCells := 0
	resolvedCells := 0
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || len(row.Routes) == 0 {
			t.Fatalf("invalid/repeated native row: %s", row.ID)
		}
		seen[row.ID] = true
		for route := range row.Routes {
			if route != "anonymous" && route != "isTest" {
				t.Fatalf("unknown route: %s", route)
			}
		}
		for _, route := range data.Routes {
			sources, ok := row.Routes[route]
			if !ok {
				if row.RouteBoundaries[route] == "" {
					t.Fatalf("unexplained missing route: %s/%s", row.ID, route)
				}
				t.Logf("route boundary %s/%s: %s", row.ID, route, row.RouteBoundaries[route])
				continue
			}
			if row.RouteBoundaries[route] != "" || len(sources.Cells) != len(data.APIVersions) {
				t.Fatalf("incomplete route: %s/%s", row.ID, route)
			}
			for _, api := range data.APIVersions {
				cell, ok := sources.Cells[api]
				resolvedCells++
				if !ok || cell.Source.ID != row.ID || (cell.Expected == nil) == (cell.Carry == nil) {
					t.Fatalf("invalid native cell: %s/%s/%s", row.ID, route, api)
				}
				if cell.Carry != nil {
					if cell.Carry.Owner == "" || cell.Carry.Reason == "" {
						t.Fatal("carry needs owner and reason")
					}
				} else {
					actualCells++
					if cell.Expected.Outcome != "runtime" && cell.Expected.Outcome != "compile-rejected" {
						t.Fatal("artifact is not a native answer")
					}
				}
				soqlBindNativeCheckSources(t, cell.Source.soqlBindNativeRoute)
			}
		}
	}
	if actualCells != data.AssertedCellCount || resolvedCells != data.ResolvedCellCount {
		t.Fatalf("assertable/resolved cells: %d/%d, want %d/%d", actualCells, resolvedCells, data.AssertedCellCount, data.ResolvedCellCount)
	}
	bases := newConformanceSchemaCache(t)
	projects := newConformanceProjectCache(t)
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, family, api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range data.Routes {
						sources, ok := row.Routes[route]
						if !ok {
							continue
						}
						t.Run(route, func(t *testing.T) {
							cell := sources.Cells[api]
							if cell.Carry != nil {
								t.Logf("carried owner=%s: %s", cell.Carry.Owner, cell.Carry.Reason)
								return
							}
							counts.TrackKind(t, route, "exact", 1)
							if route == "anonymous" {
								soqlBindNativeAnonymous(t, bases, projects, api, cell.Source, *cell.Expected)
							} else {
								soqlBindNativeTest(t, bases, projects, api, cell.Source, *cell.Expected)
							}
						})
					}
				})
			}
		})
	}
}

func soqlBindNativeValue(t *testing.T, id string, want soqlBindNativeExpected, execErr error) {
	t.Helper()
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) || runtimeErr.Type != "System.AssertException" {
		t.Fatalf("captured terminal assertion missing: %v", execErr)
	}
	marker := "C126_RESULT|" + id + "|"
	message := runtimeErr.ExceptionMessage()
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
}

func soqlBindNativeAnonymous(t *testing.T, bases *conformanceSchemaCache, projects *conformanceProjectCache, api string, source soqlBindNativeSource, want soqlBindNativeExpected) {
	t.Helper()
	declarations := soqlBindNativeAnonymousDeclarations(t, source.Anonymous)
	index := sema.WithAnonymousDeclarationContext(soqlBindNativeIndex(t, api, declarations, source.Objects, projects))
	body := []byte(source.Anonymous)
	parsed := apexast.ParseSource("NativeObservationAnonymous.cls", source.Anonymous)
	for _, decl := range parsed.Declarations {
		if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
			continue
		}
		for i := decl.Range.Start.Offset; i < decl.Range.End.Offset; i++ {
			if body[i] != '\n' && body[i] != '\r' {
				body[i] = ' '
			}
		}
	}
	// Match the product request path: declaration errors precede body errors.
	diagnostics := sema.NativeLifecycleDiagnostics(index, index.Diagnostics)
	if !soqlBindNativeHasErrors(diagnostics) {
		diagnostics = append(diagnostics, sema.AnalyzeAnonymousDeclarations(index).Diagnostics...)
	}
	if !soqlBindNativeHasErrors(diagnostics) {
		diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, string(body), api).Diagnostics...)
	}
	program, compileErr := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api, ApprovedPrefixStatements: sema.ApprovedAnonymousPrefixStatements(index, string(body), api)})
	if want.Outcome == "compile-rejected" {
		soqlBindNativeReject(t, source.soqlBindNativeRoute, api, diagnostics, compileErr)
		return
	}
	if soqlBindNativeHasErrors(diagnostics) || compileErr != nil {
		t.Fatalf("native executes; anonymous diagnostics=%v compile=%v", diagnostics, compileErr)
	}
	runtime := conformanceSchemaRequestRuntime(t, index)
	base := bases.get(t, api, "", source.Objects, func(t *testing.T) *conformanceRunner {
		return conformanceSchemaBase(t, soqlBindNativeIndex(t, api, nil, source.Objects, projects), api)
	})
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, RequestRuntime: &runtime, Org: base.orgSource})
	machine := runner.newMachine()
	_, execErr := machine.Execute(program)
	soqlBindNativeValue(t, source.ID, want, execErr)
}

func soqlBindNativeTest(t *testing.T, bases *conformanceSchemaCache, projects *conformanceProjectCache, api string, source soqlBindNativeSource, want soqlBindNativeExpected) {
	t.Helper()
	index := soqlBindNativeIndex(t, api, source.Classes, source.Objects, projects)
	analysis := sema.Analyze(index)
	// The analyzer includes and refines index diagnostics. Match the product
	// compile gate without prepending raw parser errors ahead of native text.
	diagnostics := analysis.Diagnostics
	if want.Outcome == "compile-rejected" {
		soqlBindNativeReject(t, source.soqlBindNativeRoute, api, diagnostics, nil)
		return
	}
	if soqlBindNativeHasErrors(diagnostics) {
		t.Fatalf("native executes; named diagnostics=%v", diagnostics)
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
	base := bases.get(t, api, "", source.Objects, func(t *testing.T) *conformanceRunner {
		return conformanceSchemaBase(t, soqlBindNativeIndex(t, api, nil, source.Objects, projects), api)
	})
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true, Org: base.orgSource})
	machine := runner.newMachine()
	if err := machine.RegisterMethod(methods[key]); err != nil {
		t.Fatal(err)
	}
	machine.EnableTestContext()
	_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)

	soqlBindNativeValue(t, source.ID, want, execErr)
}

func soqlBindNativeHasErrors(items []diagnostic.Diagnostic) bool {
	for _, d := range items {
		if d.Severity == diagnostic.Error {
			return true
		}
	}
	return false
}

func soqlBindNativeCheckSources(t *testing.T, source soqlBindNativeRoute) {
	t.Helper()
	values := map[string]string{}
	if source.Anonymous != "" {
		values["anonymous"] = source.Anonymous
	}
	for prefix, entries := range map[string]map[string]string{"class:": source.Classes, "object:": source.Objects, "setupClass:": source.SetupClasses, "setupObject:": source.SetupObjects} {
		for name, text := range entries {
			values[prefix+name] = text
		}
	}
	if len(values) == 0 || len(values) != len(source.SourceSHA256) {
		t.Fatal("captured source hash matrix incomplete")
	}
	for name, text := range values {
		sum := sha256.Sum256([]byte(text))
		if hex.EncodeToString(sum[:]) != source.SourceSHA256[name] {
			t.Fatalf("captured Apex/metadata bytes changed: %s/%s", source.ID, name)
		}
	}
}

func soqlBindNativeAnonymousDeclarations(t *testing.T, source string) map[string]string {
	t.Helper()
	parsed := apexast.ParseSource("NativeBoundaryAnonymous.cls", source)
	declarations := map[string]string{}
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
		declarations["GladeAnonymous"+decl.Name] = string(padding) + source[start:end]
	}
	return declarations
}

func soqlBindNativeIndex(t *testing.T, api string, classes, objects map[string]string, projects ...*conformanceProjectCache) typesys.Index {
	t.Helper()
	if len(projects) != 0 {
		files := make(map[string]string, 2*len(classes)+len(objects))
		for name, source := range classes {
			path := filepath.Join("force-app/main/default/classes", name+".cls")
			files[path] = source
			files[path+"-meta.xml"] = "<ApexClass><apiVersion>" + api + "</apiVersion></ApexClass>"
		}
		for name, source := range objects {
			files[filepath.Join("force-app/main/default/objects", name, name+".object-meta.xml")] = source
		}
		return projects[0].index(t, api, files)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	for name, source := range objects {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml"), source)
	}
	return loadTestIndex(t, root)
}

func soqlBindNativeReject(t *testing.T, source soqlBindNativeRoute, api string, diagnostics []diagnostic.Diagnostic, compileErr error) {
	t.Helper()
	want := source.NativeDiagnostics
	if perAPI := source.NativeDiagnosticsByAPI; len(perAPI) != 0 {
		want = perAPI[api]
	}
	if len(want) == 0 || want[0].Problem == "" {
		t.Fatal("missing exact native diagnostic")
	}
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
	if len(messages) == 0 || messages[0] != want[0].Problem {
		t.Fatalf("native rejection <%s>; Glade diagnostics=%v compile=%v", want[0].Problem, messages, compileErr)
	}
	t.Logf("native diagnostic count=%d; exact primary asserted; deployment cascades retained as evidence", len(want))
}
