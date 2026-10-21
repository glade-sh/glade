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

type nativeBoundaryExpected struct {
	Outcome string `json:"outcome"`
	Value   string `json:"value,omitempty"`
}

type nativeBoundaryDiagnostic struct {
	Problem string `json:"problem"`
}

type nativeBoundaryRoute struct {
	RuntimeAPIVersion      string                                `json:"runtimeAPIVersion,omitempty"`
	ID                     string                                `json:"id"`
	Anonymous              string                                `json:"anonymous,omitempty"`
	Classes                map[string]string                     `json:"classes,omitempty"`
	Objects                map[string]string                     `json:"objects,omitempty"`
	SetupClasses           map[string]string                     `json:"setupClasses,omitempty"`
	SetupObjects           map[string]string                     `json:"setupObjects,omitempty"`
	GroupMetadata          map[string]string                     `json:"groupMetadata,omitempty"`
	SharingRuleMetadata    map[string]string                     `json:"sharingRuleMetadata,omitempty"`
	RoleMetadata           map[string]string                     `json:"roleMetadata,omitempty"`
	ObservationMethod      string                                `json:"observationMethod,omitempty"`
	Tests                  []string                              `json:"tests,omitempty"`
	TestMethod             string                                `json:"testMethod,omitempty"`
	Stage                  string                                `json:"stage"`
	NativeSetupRejection   string                                `json:"nativeSetupRejection,omitempty"`
	NativeDiagnostics      []nativeBoundaryDiagnostic            `json:"nativeDiagnostics,omitempty"`
	NativeDiagnosticsByAPI map[string][]nativeBoundaryDiagnostic `json:"nativeDiagnosticsByAPI,omitempty"`
	SourceSHA256           map[string]string                     `json:"sourceSha256"`
	FixedClassAPIVersions  map[string]string                     `json:"fixedClassAPIVersions,omitempty"`
}

type nativeBoundaryRow struct {
	ID            string                            `json:"id"`
	Expected      *nativeBoundaryExpected           `json:"expected,omitempty"`
	ExpectedByAPI map[string]nativeBoundaryExpected `json:"expectedByAPI,omitempty"`
	Routes        map[string]nativeBoundaryRoute    `json:"routes"`
}

// runNativeBoundaryConformance retains captured source, including the terminal
// observation assertion. It delegates runtime isolation to conformanceRunner.
// Compilation remains at the call site, as in Compression and AbstractOverride.
// A setup rejection checks the helper compilation only; anonymous execution was
// never observed. Full native diagnostic arrays are retained in JSON; the exact
// first route-visible diagnostic is asserted, not deployment cascade metadata.
func runNativeBoundaryConformance(t *testing.T, file, family string, shapeCount int) {
	t.Helper()
	var data struct {
		APIVersions []string            `json:"apiVersions"`
		Rows        []nativeBoundaryRow `json:"rows"`
	}
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", file))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != shapeCount || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatalf("native matrix: %d shapes, APIs %v", len(data.Rows), data.APIVersions)
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		if row.ID == "" || seen[row.ID] || (len(row.Routes) < 1 || len(row.Routes) > 2) || (row.Expected == nil) == (len(row.ExpectedByAPI) == 0) {
			t.Fatalf("invalid/repeated native row: %s", row.ID)
		}
		seen[row.ID] = true
		for route, source := range row.Routes {
			if (route != "anonymous" && route != "isTest") || source.ID == "" {
				t.Fatalf("invalid captured route: %s/%s", row.ID, route)
			}
			nativeBoundaryCheckSources(t, source)
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, family, api)
			for _, row := range data.Rows {
				t.Run(row.ID, func(t *testing.T) {
					want := row.Expected
					if want == nil {
						value, ok := row.ExpectedByAPI[api]
						if !ok {
							t.Fatal("missing per-API native expectation")
						}
						want = &value
					}
					for _, route := range []string{"anonymous", "isTest"} {
						if _, captured := row.Routes[route]; !captured {
							continue
						}
						t.Run(route, func(t *testing.T) {
							counts.TrackKind(t, route, "exact", 1)
							source := row.Routes[route]
							classes, objects := source.Classes, source.Objects
							if route == "anonymous" {
								classes, objects = source.SetupClasses, source.SetupObjects
							}
							index := nativeBoundaryIndexWithClassAPIs(t, api, classes, objects, source.FixedClassAPIVersions, nativeBoundaryMetadata{roles: source.RoleMetadata, groups: source.GroupMetadata, sharingRules: source.SharingRuleMetadata})
							analysis := sema.Analyze(index)
							diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
							if source.Stage == "setup-rejected" {
								if route != "anonymous" || source.NativeSetupRejection == "" || want.Outcome != "reject" {
									t.Fatal("invalid native setup rejection")
								}
								nativeBoundaryReject(t, source, api, diagnostics, nil)
								return
							}
							if route == "anonymous" {
								if index.HasErrors() || analysis.HasErrors() {
									t.Fatalf("native setup accepted; Glade rejected: %v", diagnostics)
								}
								// Anonymous-local declarations use the same transient
								// analysis path as AbstractOverride, not named semantics.
								declarations := nativeBoundaryAnonymousDeclarations(t, source.Anonymous)
								if len(declarations) != 0 {
									if want.Outcome != "reject" || len(classes) != 0 || len(objects) != 0 {
										t.Fatal("this capture has only isolated rejected anonymous declarations")
									}
									transient := sema.WithAnonymousDeclarationContext(nativeBoundaryIndex(t, api, declarations, nil))
									local := sema.AnalyzeAnonymousDeclarationsInContext(transient, transient)
									nativeBoundaryReject(t, source, api, local.Diagnostics, nil)
									return
								}
								anonymous := sema.AnalyzeAnonymous(index, source.Anonymous, api)
								program, compileErr := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
								if want.Outcome == "reject" {
									nativeBoundaryReject(t, source, api, anonymous.Diagnostics, compileErr)
									return
								}
								if anonymous.HasErrors() || compileErr != nil {
									t.Fatalf("native executes; anonymous diagnostics=%v compile=%v", anonymous.Diagnostics, compileErr)
								}
								org := orgFromIndex(index)
								org.APIVersion = api
								runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
								_, execErr := runner.execute(program)
								nativeBoundaryValue(t, source.ID, *want, execErr)
								return
							}
							if want.Outcome == "reject" {
								nativeBoundaryReject(t, source, api, diagnostics, nil)
								return
							}
							if index.HasErrors() || analysis.HasErrors() {
								t.Fatalf("native executes; named diagnostics=%v", diagnostics)
							}
							if len(source.Tests) != 1 {
								t.Fatal("one captured test class required")
							}
							method := source.ObservationMethod
							if method == "" {
								method = "observe"
							}
							cases := Discover(index, Options{SelectedClasses: source.Tests, SelectedMethod: method})
							if len(cases) != 1 {
								t.Fatalf("named observation discovery: %d", len(cases))
							}
							methods, methodErrors := compileTestMethods(cases)
							programs, programErrors := compileTestInvokePrograms(cases)
							key := testCaseKey(cases[0])
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("named compile: %v %v", methodErrors[key], programErrors[key])
							}
							org := orgFromIndex(index)
							org.APIVersion = api
							runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
							machine := runner.newMachine()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							machine.EnableTestContext()
							_, execErr := machine.ExecuteInClass(programs[key], cases[0].ClassName)
							nativeBoundaryValue(t, source.ID, *want, execErr)
						})
					}
				})
			}
		})
	}
}

func nativeBoundaryCheckSources(t *testing.T, source nativeBoundaryRoute) {
	t.Helper()
	values := map[string]string{}
	if source.Anonymous != "" {
		values["anonymous"] = source.Anonymous
	}
	for prefix, entries := range map[string]map[string]string{"class:": source.Classes, "object:": source.Objects, "setupClass:": source.SetupClasses, "setupObject:": source.SetupObjects, "role:": source.RoleMetadata, "group:": source.GroupMetadata, "sharingRule:": source.SharingRuleMetadata} {
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

func nativeBoundaryAnonymousDeclarations(t *testing.T, source string) map[string]string {
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

func nativeBoundaryIndex(t *testing.T, api string, classes, objects map[string]string) typesys.Index {
	t.Helper()
	return nativeBoundaryIndexWithClassAPIs(t, api, classes, objects, nil)
}

type nativeBoundaryMetadata struct {
	roles, groups, sharingRules map[string]string
}

func nativeBoundaryIndexWithClassAPIs(t *testing.T, api string, classes, objects, fixedClassAPIs map[string]string, metadata ...nativeBoundaryMetadata) typesys.Index {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	for name, source := range classes {
		classAPI := api
		if fixed := fixedClassAPIs[name]; fixed != "" {
			classAPI = fixed
		}
		path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+classAPI+"</apiVersion></ApexClass>")
	}
	for name, source := range objects {
		writeFile(t, filepath.Join(root, "force-app/main/default/objects", name, name+".object-meta.xml"), source)
	}
	for _, setup := range metadata {
		for name, source := range setup.sharingRules {
			writeFile(t, filepath.Join(root, "force-app/main/default/sharingRules", name+".sharingRules-meta.xml"), source)
		}
		for kind, files := range map[string]map[string]string{"role": setup.roles, "group": setup.groups} {
			for name, source := range files {
				writeFile(t, filepath.Join(root, "force-app/main/default", kind+"s", name+"."+kind+"-meta.xml"), source)
			}
		}
	}
	return loadTestIndex(t, root)
}

func nativeBoundaryReject(t *testing.T, source nativeBoundaryRoute, api string, diagnostics []diagnostic.Diagnostic, compileErr error) {
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

func nativeBoundaryValue(t *testing.T, id string, want nativeBoundaryExpected, execErr error) {
	t.Helper()
	if want.Outcome != "runtime" {
		t.Fatal("unsupported native outcome")
	}
	var runtimeErr *vm.RuntimeError
	if execErr == nil || !errors.As(execErr, &runtimeErr) {
		t.Fatalf("captured terminal assertion missing: %v", execErr)
	}
	if runtimeErr.Type != "System.AssertException" {
		t.Fatalf("observation transport must be the terminal assertion: %v", execErr)
	}
	marker := "P|" + id + "|"
	message := runtimeErr.ExceptionMessage()
	_, observed, found := strings.Cut(message, marker)
	if !found || strings.Count(message, marker) != 1 || observed != want.Value {
		t.Fatalf("native value <%s>; Glade exception <%s>", want.Value, message)
	}
}
