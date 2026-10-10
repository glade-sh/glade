package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

type overloadApplicabilityCase struct {
	ID, Code, Expected                  string
	Compile                             bool
	Declarations, AnonymousDeclarations map[string]string
}

type overloadApplicabilityObservation struct {
	APIVersion                             string `json:"apiVersion"`
	Route, ID, Observed, Source, ClassName string
}

type overloadApplicabilityFixture struct {
	APIVersions                                        []string `json:"apiVersions"`
	Declarations, AnonymousDeclarations                map[string]string
	RuntimePrelude, NativeCaptureStatus                string
	Cases                                              []overloadApplicabilityCase
	NativeObservations                                 []overloadApplicabilityObservation
	PreservedNativeFixture                             string
	PreservedNativeRejections, PreservedNativeControls []string
}

func readOverloadApplicabilityFixture(t *testing.T, name string) overloadApplicabilityFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata/conformance", name))
	if err != nil {
		t.Fatal(err)
	}
	var fixture overloadApplicabilityFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func overloadApplicabilityProject(t *testing.T, declarations map[string]string, api string) project.Project {
	t.Helper()
	p := project.Project{Root: t.TempDir(), SourceAPIVersion: api}
	names := make([]string, 0, len(declarations))
	for name := range declarations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(p.Root, name+".cls")
		writeFile(t, path, declarations[name])
		writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
		p.ApexFiles = append(p.ApexFiles, path)
	}
	return p
}

func overloadApplicabilityIndex(t *testing.T, declarations map[string]string, api string) typesys.Index {
	t.Helper()
	index := typesys.Build(overloadApplicabilityProject(t, declarations, api), gladeschema.Schema{})
	if index.HasErrors() {
		t.Fatalf("declarations: %v", index.Diagnostics)
	}
	return index
}

// Lift captured transient declarations into the index while retaining every
// source line in both the declarations and the anonymous statement body.
func overloadAnonymousIndex(t *testing.T, source, api string) (typesys.Index, string) {
	t.Helper()
	parsed := apexast.NewParser().ParseSource("OverloadAnonymous.cls", source)
	declarations := map[string]string{}
	masked := []byte(source)
	for _, decl := range parsed.Declarations {
		switch decl.Kind {
		case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
		default:
			continue
		}
		start, end := decl.Range.Start.Offset, decl.Range.End.Offset
		if start < 0 || end <= start || end > len(source) {
			t.Fatalf("invalid captured anonymous declaration range: %s", decl.Name)
		}
		declarations[decl.Name] = strings.Repeat("\n", strings.Count(source[:start], "\n")) + source[start:end]
		for i := start; i < end; i++ {
			if masked[i] != '\n' && masked[i] != '\r' {
				masked[i] = ' '
			}
		}
	}
	index := sema.WithAnonymousDeclarationContext(overloadApplicabilityIndex(t, declarations, api))
	return index, string(masked)
}

func overloadCompileText(diagnostics []diagnostic.Diagnostic, anonymous bool) string {
	var messages []string
	for _, d := range diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		message := d.Message
		if d.NativeMessage != "" {
			message = d.NativeMessage
		}
		if anonymous {
			line := 0
			if d.Range != nil {
				line = d.Range.Start.Line
			}
			if d.NativeLine != nil {
				line = *d.NativeLine
			}
			message = fmt.Sprintf("line %d: %s", line, message)
		}
		messages = append(messages, message)
	}
	if len(messages) == 0 {
		return "compiled"
	}
	return "COMPILE_ERROR\t" + strings.Join(messages, "\\n")
}

func TestOverloadApplicabilityOrgConformance(t *testing.T) {
	data := readOverloadApplicabilityFixture(t, "overload_applicability_regression.json")
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.Cases) != 15 || data.RuntimePrelude == "" {
		t.Fatalf("native overload fixture shape: APIs=%v rows=%d", data.APIVersions, len(data.Cases))
	}
	byID := make(map[string]overloadApplicabilityCase)
	for i, row := range data.Cases {
		id := fmt.Sprintf("R%03d", i+1)
		if i >= 8 {
			id = fmt.Sprintf("C%03d", i-7)
		}
		if row.ID != id || row.Code == "" || row.Compile != (i >= 8) {
			t.Fatalf("invalid native overload case: %#v", row)
		}
		byID[row.ID] = row
	}
	native := make(map[string]overloadApplicabilityObservation)
	for _, observed := range data.NativeObservations {
		key := observed.APIVersion + "/" + observed.Route + "/" + observed.ID
		row, exists := byID[observed.ID]
		_, duplicate := native[key]
		if !exists || duplicate || observed.Observed == "" ||
			(observed.APIVersion != "62.0" && observed.APIVersion != "67.0") ||
			(observed.Route != "anonymous" && observed.Route != "isTest") ||
			row.Compile != strings.HasPrefix(observed.Observed, "COMPILE_ERROR\t") ||
			(row.Compile || observed.Route == "isTest") && observed.Source == "" ||
			observed.Route == "isTest" && observed.ClassName == "" {
			t.Fatalf("invalid native overload observation: %#v", observed)
		}
		native[key] = observed
	}
	for _, api := range data.APIVersions {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, row := range data.Cases {
				if _, found := native[api+"/"+route+"/"+row.ID]; !found {
					t.Fatalf("missing native overload observation: %s/%s/%s", api, route, row.ID)
				}
			}
		}
	}
	if len(native) != 60 {
		t.Fatalf("native overload matrix: %d observations, want 60", len(native))
	}
	assertion := func(id, expected string) string {
		return "String expectedText=" + conformanceApexString(expected) + "; String observedText=String.valueOf(r); " +
			"System.assert(expectedText.equals(observedText),'" + id + " expected <'+expectedText+'> actual <'+observedText+'>');"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			anonymousIndex, _ := overloadAnonymousIndex(t, data.RuntimePrelude, api)
			if analysis := sema.AnalyzeAnonymousDeclarations(anonymousIndex); analysis.HasErrors() {
				t.Fatalf("anonymous helper semantics: %v", analysis.Diagnostics)
			}
			anonymousRunner := newConformanceRunner(t, anonymousIndex, conformanceRunnerOptions{LinkProject: true})
			namedSources := make(map[string]string)
			var namedClasses []string
			for _, row := range data.Cases {
				if row.Compile {
					continue
				}
				observed := native[api+"/isTest/"+row.ID]
				terminal := "System.assert(false,'P|" + row.ID + "|'+String.valueOf(r));"
				if strings.Count(observed.Source, terminal) != 1 {
					t.Fatalf("named source lacks exact observation envelope: %s", row.ID)
				}
				namedSources[observed.ClassName] = strings.Replace(observed.Source, terminal, assertion(row.ID, observed.Observed), 1)
				namedClasses = append(namedClasses, observed.ClassName)
			}
			namedIndex := overloadApplicabilityIndex(t, namedSources, api)
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named helper semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{LinkProject: true})
			namedCases := Discover(namedIndex, Options{SelectedClasses: namedClasses})
			if len(namedCases) != 8 {
				t.Fatalf("named overload discovery: %d, want 8", len(namedCases))
			}
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			selected := make(map[string]TestCase)
			for _, tc := range namedCases {
				key := testCaseKey(tc)
				observed, found := native[api+"/isTest/"+tc.MethodName]
				_, duplicate := selected[tc.MethodName]
				if !found || byID[tc.MethodName].Compile || duplicate || tc.ClassName != observed.ClassName {
					t.Fatalf("unexpected named overload method: %s", key)
				}
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named overload compilation %s: %v %v", key, methodErrors[key], programErrors[key])
				}
				selected[tc.MethodName] = tc
			}
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						observed := native[api+"/anonymous/"+row.ID]
						if row.Compile {
							index, body := overloadAnonymousIndex(t, observed.Source, api)
							diagnostics := sema.AnalyzeAnonymousDeclarations(index).Diagnostics
							diagnostics = append(diagnostics, sema.AnalyzeAnonymous(index, body, api).Diagnostics...)
							if actual := overloadCompileText(diagnostics, true); actual != observed.Observed {
								t.Fatalf("expected <%s> actual <%s>", observed.Observed, actual)
							}
							return
						}
						body := "Object r; try {" + row.Code + "} catch(Exception e) {r='EXC|'+e.getTypeName()+'|'+e.getMessage();} " + assertion(row.ID, observed.Observed)
						if analysis := sema.AnalyzeAnonymous(anonymousIndex, body, api); analysis.HasErrors() {
							t.Fatalf("anonymous overload semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := anonymousRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						observed := native[api+"/isTest/"+row.ID]
						if row.Compile {
							index := overloadApplicabilityIndex(t, map[string]string{observed.ClassName: observed.Source}, api)
							if actual := overloadCompileText(sema.Analyze(index).Diagnostics, false); actual != observed.Observed {
								t.Fatalf("expected <%s> actual <%s>", observed.Observed, actual)
							}
							return
						}
						tc := selected[row.ID]
						key := testCaseKey(tc)
						machine := namedRunner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(programs[key], tc.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}

func requireOverloadRuntimeAmbiguity(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ambiguous overload") {
		t.Fatalf("runtime must reject as ambiguous, got %v", err)
	}
}

func requireOverloadSemanticAmbiguity(t *testing.T, analysis sema.Result) {
	t.Helper()
	for _, diagnostic := range analysis.Diagnostics {
		if diagnostic.Code == "GLADESEMA022" {
			return
		}
	}
	t.Fatalf("semantics must reject an ambiguous call, got %v", analysis.Diagnostics)
}

func assertAnonymousOverloadRejection(t *testing.T, row overloadApplicabilityCase, anonymousDeclarations map[string]string, api string) {
	t.Helper()
	t.Run("anonymous", func(t *testing.T) {
		index := overloadApplicabilityIndex(t, anonymousDeclarations, api)
		if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
			t.Fatalf("declaration semantics: %v", analysis.Diagnostics)
		}
		t.Run("semantics", func(t *testing.T) {
			requireOverloadSemanticAmbiguity(t, sema.AnalyzeAnonymous(index, row.Code, api))
		})
		t.Run("raw-runtime", func(t *testing.T) {
			// Always bypass the semantic gate here. Its rejection cannot stand
			// in for the shared runtime overload selector's own result.
			program, err := vm.CompileAnonymousWithOptions(row.Code, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatalf("raw compilation: %v", err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			_, err = runner.execute(program)
			requireOverloadRuntimeAmbiguity(t, err)
		})
	})
}

// Use the unchanged anonymous captures on their observed route. In particular,
// R171's semantic rejection must not hide Object/List raw-dispatch selection.
func TestOverloadApplicabilityPreservesNativeAmbiguity(t *testing.T) {
	regression := readOverloadApplicabilityFixture(t, "overload_applicability_regression.json")
	native := readOverloadApplicabilityFixture(t, regression.PreservedNativeFixture)
	byID := make(map[string]overloadApplicabilityCase, len(native.Cases))
	for _, row := range native.Cases {
		byID[row.ID] = row
	}
	if strings.Join(regression.PreservedNativeRejections, ",") != "C009,C010,R167,R168,R169,R170,R171" {
		t.Fatalf("native rejection selection: %v", regression.PreservedNativeRejections)
	}
	for _, api := range native.APIVersions {
		t.Run(api, func(t *testing.T) {
			for _, id := range regression.PreservedNativeRejections {
				row, found := byID[id]
				if !found || !strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
					t.Fatalf("missing captured rejection: %s", id)
				}
				row.Code = "Object r; " + row.Code
				t.Run(id, func(t *testing.T) {
					assertAnonymousOverloadRejection(t, row, native.AnonymousDeclarations, api)
				})
			}
		})
	}
}

// Anonymous captures of typed-null and Object-held controls bound the new
// bare-null rule on that route.
func TestOverloadApplicabilityPreservesNativeControls(t *testing.T) {
	regression := readOverloadApplicabilityFixture(t, "overload_applicability_regression.json")
	native := readOverloadApplicabilityFixture(t, regression.PreservedNativeFixture)
	byID := make(map[string]overloadApplicabilityCase, len(native.Cases))
	for _, row := range native.Cases {
		byID[row.ID] = row
	}
	if strings.Join(regression.PreservedNativeControls, ",") != "R142,R143,R144,R154,R156,R160,R164,R165,R166" {
		t.Fatalf("native control selection: %v", regression.PreservedNativeControls)
	}
	for _, api := range native.APIVersions {
		t.Run(api, func(t *testing.T) {
			index := overloadApplicabilityIndex(t, native.AnonymousDeclarations, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			for _, id := range regression.PreservedNativeControls {
				row, found := byID[id]
				if !found || row.Expected == "" || strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
					t.Fatalf("missing captured control: %s", id)
				}
				body := "Object r; " + row.Code + " String expectedText=" + conformanceApexString(row.Expected) + "; String observedText=String.valueOf(r); System.assert(expectedText.equals(observedText)," + conformanceApexString(row.ID) + ");"
				t.Run(id+"/anonymous", func(t *testing.T) {
					if analysis := sema.AnalyzeAnonymous(index, body, api); analysis.HasErrors() {
						t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
