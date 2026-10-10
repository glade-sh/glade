package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Every API/route/case expectation comes from its own native observation.
// Anonymous controls preserve the test-context boundary; named controls
// exercise the declared return type and Object-boxed rejection twins.
func TestStubDeclaredReturnType(t *testing.T) {
	var data struct {
		APIVersions           []string          `json:"apiVersions"`
		ExpectationStatus     string            `json:"expectationStatus"`
		Declarations          map[string]string `json:"declarations"`
		AnonymousDeclarations map[string]string `json:"anonymousDeclarations"`
		TestClass             string            `json:"testClass"`
		TestHead              string            `json:"testHead"`
		Cases                 []struct {
			ID            string `json:"id"`
			Code          string `json:"code"`
			AnonymousCode string `json:"anonymousCode"`
		} `json:"cases"`
		NativeObservations []struct {
			APIVersion string `json:"apiVersion"`
			Route      string `json:"route"`
			ID         string `json:"id"`
			Observed   string `json:"observed"`
		} `json:"nativeObservations"`
		NativeSourceReferences []struct {
			APIVersion, Route string
		} `json:"nativeSourceReferences"`
	}
	raw, err := os.ReadFile("testdata/conformance/stub_return_type.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.ExpectationStatus != "native captured" || len(data.Cases) != 7 || data.TestClass == "" || data.TestHead == "" {
		t.Fatalf("invalid stub return fixture: APIs=%v cases=%d class=%q", data.APIVersions, len(data.Cases), data.TestClass)
	}
	sources := make(map[string]bool)
	for _, source := range data.NativeSourceReferences {
		key := source.APIVersion + "/" + source.Route
		if sources[key] || (source.APIVersion != "62.0" && source.APIVersion != "67.0") ||
			(source.Route != "anonymous" && source.Route != "isTest") {
			t.Fatalf("invalid native stub return source reference: %#v", source)
		}
		sources[key] = true
	}
	if len(sources) != 4 {
		t.Fatalf("native stub return sources: %d, want 4", len(sources))
	}
	if len(data.Declarations) != 1 || data.Declarations["StubReturnTarget"] == "" || len(data.AnonymousDeclarations) != 2 ||
		data.AnonymousDeclarations["StubReturnTarget"] != data.Declarations["StubReturnTarget"] || data.AnonymousDeclarations["StubReturnAnonymous"] == "" {
		t.Fatal("invalid stub return route declarations")
	}
	seen := make(map[string]bool)
	for i, row := range data.Cases {
		if row.ID != fmt.Sprintf("R%03d", i+1) || seen[row.ID] || row.Code == "" ||
			row.AnonymousCode != strings.ReplaceAll(row.Code, "stub()", "StubReturnAnonymous.stub()") {
			t.Fatalf("invalid stub return row: %#v", row)
		}
		seen[row.ID] = true
	}
	native := make(map[string]string)
	for _, row := range data.NativeObservations {
		key := row.APIVersion + "/" + row.Route + "/" + row.ID
		if _, duplicate := native[key]; duplicate || !seen[row.ID] ||
			(row.APIVersion != "62.0" && row.APIVersion != "67.0") ||
			(row.Route != "anonymous" && row.Route != "isTest") || row.Observed == "" || row.Observed == "?" || strings.HasPrefix(row.Observed, "COMPILE_ERROR") {
			t.Fatalf("invalid native stub return observation: %#v", row)
		}
		native[key] = row.Observed
	}
	var missing []string
	for _, api := range data.APIVersions {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, row := range data.Cases {
				key := api + "/" + route + "/" + row.ID
				if _, captured := native[key]; !captured {
					missing = append(missing, key)
				}
			}
		}
	}
	if len(native) != 28 || len(missing) != 0 {
		t.Fatalf("native stub return matrix incomplete (%s): captured %d/28; missing %s", data.ExpectationStatus, len(native), strings.Join(missing, ", "))
	}
	body := func(id, code, expected string) string {
		return "Object r; try {" + code + "} catch(Exception e) { r='EXC|'+e.getTypeName()+'|'+e.getMessage(); } " +
			"String expectedText=" + conformanceApexString(expected) + "; String observedText=String.valueOf(r); " +
			"System.assert(expectedText.equals(observedText),'" + id + " expected <'+expectedText+'> actual <'+observedText+'>');"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "test runner", api, "stub-return")
			buildIndex := func(declarations map[string]string, testSource string) typesys.Index {
				t.Helper()
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				writeClass := func(name, source string) {
					path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>\n")
				}
				for name, source := range declarations {
					writeClass(name, source)
				}
				if testSource != "" {
					writeClass(data.TestClass, testSource)
				}
				index := loadTestIndex(t, root)
				if index.HasErrors() {
					t.Fatalf("stub return declaration parser: %v", index.Diagnostics)
				}
				if analysis := sema.Analyze(index); analysis.HasErrors() {
					t.Fatalf("stub return declaration semantics: %v", analysis.Diagnostics)
				}
				return index
			}
			anonymousIndex := buildIndex(data.AnonymousDeclarations, "")
			anonymousRunner := newConformanceRunner(t, anonymousIndex, conformanceRunnerOptions{LinkProject: true})
			var source strings.Builder
			source.WriteString(data.TestHead)
			for _, row := range data.Cases {
				fmt.Fprintf(&source, "    @IsTest static void %s() { %s }\n", row.ID, body(row.ID, row.Code, native[api+"/isTest/"+row.ID]))
			}
			source.WriteString("}\n")
			namedIndex := buildIndex(data.Declarations, source.String())
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{LinkProject: true})
			testCases := Discover(namedIndex, Options{SelectedClasses: []string{data.TestClass}})
			if len(testCases) != len(data.Cases) {
				t.Fatalf("stub return named discovery: %d methods, want %d", len(testCases), len(data.Cases))
			}
			methods, methodErrors := compileTestMethods(testCases)
			programs, programErrors := compileTestInvokePrograms(testCases)
			byID := make(map[string]TestCase)
			for _, tc := range testCases {
				if _, duplicate := byID[tc.MethodName]; duplicate || tc.ClassName != data.TestClass || !seen[tc.MethodName] {
					t.Fatalf("unexpected stub return named method: %s.%s", tc.ClassName, tc.MethodName)
				}
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("stub return named compilation %s: %v %v", key, methodErrors[key], programErrors[key])
				}
				byID[tc.MethodName] = tc
			}
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					counts.Run(t, "anonymous", row.ID, "anonymous", func(t *testing.T) {
						anonymous := body(row.ID, row.AnonymousCode, native[api+"/anonymous/"+row.ID])
						if analysis := sema.AnalyzeAnonymous(anonymousIndex, anonymous, api); analysis.HasErrors() {
							t.Fatalf("stub return anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := anonymousRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					counts.Run(t, "isTest", row.ID, "isTest", func(t *testing.T) {
						tc := byID[row.ID]
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
