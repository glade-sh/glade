package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Every case has an independently captured anonymous and @IsTest result at
// both API endpoints. Comparisons preserve case, whitespace and exception text.
func TestValueEdgesNativeConformance(t *testing.T) {
	type namedSource struct {
		ClassName, Source string
	}
	type framedNamedObservation struct {
		ID, Code, Expected, ClassName, Source string
	}
	type row struct {
		ID, Group, Code    string
		Compile            bool
		ExpectedByRouteAPI map[string]map[string]string
		NamedSourcesByAPI  map[string]namedSource
		FramedNamedByAPI   map[string]framedNamedObservation
	}
	var data struct {
		APIVersions       []string `json:"apiVersions"`
		ExpectationStatus string
		Declarations      map[string]string
		Cases             []row
	}
	raw, err := os.ReadFile("testdata/conformance/corpus_rt_value_edges.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.ExpectationStatus != "native captured" || len(data.Cases) != 68 || len(data.Declarations) != 0 {
		t.Fatalf("native value-edge fixture shape: APIs=%v status=%q cases=%d declarations=%d", data.APIVersions, data.ExpectationStatus, len(data.Cases), len(data.Declarations))
	}
	observations, framedObservations := 0, 0
	for i, tc := range data.Cases {
		if tc.ID != fmt.Sprintf("R%03d", i+1) || tc.Group == "" || tc.Code == "" || tc.Compile || len(tc.ExpectedByRouteAPI) != 2 || len(tc.NamedSourcesByAPI) != 2 {
			t.Fatalf("invalid native value-edge case: %#v", tc)
		}
		for _, route := range []string{"anonymous", "isTest"} {
			values, found := tc.ExpectedByRouteAPI[route]
			if !found || len(values) != 2 {
				t.Fatalf("%s missing native route: %s", tc.ID, route)
			}
			for _, api := range data.APIVersions {
				expected, found := values[api]
				if !found || expected == "" || expected == "?" || strings.HasPrefix(expected, "COMPILE_ERROR") {
					t.Fatalf("%s/%s/%s missing native runtime observation: %q", api, route, tc.ID, expected)
				}
				observations++
			}
		}
		for _, api := range data.APIVersions {
			source, found := tc.NamedSourcesByAPI[api]
			if !found || source.ClassName == "" || source.Source == "" ||
				!strings.Contains(source.Source, "@IsTest private class "+source.ClassName+" {") ||
				!strings.Contains(source.Source, "@IsTest static void "+tc.ID+"() {") ||
				strings.Count(source.Source, "\n"+tc.Code+"\n") != 1 {
				t.Fatalf("%s/%s missing source-bound named observation", api, tc.ID)
			}
		}
		if tc.ID == "R039" {
			if len(tc.FramedNamedByAPI) != 2 {
				t.Fatal("R039 requires both native framed named observations")
			}
			for _, api := range data.APIVersions {
				framed, found := tc.FramedNamedByAPI[api]
				if !found || framed.ID != "R039F" || framed.Code != tc.Code+"r='['+String.valueOf(r)+']';" ||
					framed.Expected != "["+tc.ExpectedByRouteAPI["anonymous"][api]+"]" ||
					framed.Expected != "["+tc.ExpectedByRouteAPI["isTest"][api]+" ]" ||
					framed.ClassName == "" ||
					!strings.Contains(framed.Source, "@IsTest private class "+framed.ClassName+" {") ||
					!strings.Contains(framed.Source, "@IsTest static void "+framed.ID+"() {") ||
					strings.Count(framed.Source, "\n"+framed.Code+"\n") != 1 {
					t.Fatalf("%s/R039 missing source-bound framed native observation", api)
				}
				framedObservations++
			}
		} else if len(tc.FramedNamedByAPI) != 0 {
			t.Fatalf("unexpected framed observation: %s", tc.ID)
		}
	}
	if observations != 272 || framedObservations != 2 {
		t.Fatalf("native value-edge matrix: %d primary and %d framed observations, want 272 and 2", observations, framedObservations)
	}
	namedObservation := func(tc row, api string) (namedSource, string, string) {
		if tc.ID == "R039" {
			// The original terminal assertion loses its final payload space.
			// Preserve that raw observation above and execute the independently
			// captured framed operation with an exact, untrimmed comparison.
			framed := tc.FramedNamedByAPI[api]
			return namedSource{ClassName: framed.ClassName, Source: framed.Source}, framed.ID, framed.Expected
		}
		return tc.NamedSourcesByAPI[api], tc.ID, tc.ExpectedByRouteAPI["isTest"][api]
	}
	assertion := func(id, expected string) string {
		return "String expectedText=" + conformanceApexString(expected) + "; String observedText=corpusObserved; " +
			"System.assert(expectedText.equals(observedText)," + conformanceApexString(id) + "+' expected <'+expectedText+'> actual <'+observedText+'>');"
	}
	anonymousBody := func(tc row, expected string) string {
		return "String corpusObserved; try { Object r;\n" + tc.Code +
			"\ncorpusObserved=String.valueOf(r); } catch(Exception e) { corpusObserved='EXC|'+e.getTypeName()+'|'+e.getMessage(); }\n" + assertion(tc.ID, expected)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classRows := make(map[string]string, len(data.Cases))
			methodNames := make(map[string]string, len(data.Cases))
			var classNames []string
			for _, tc := range data.Cases {
				source, method, expected := namedObservation(tc, api)
				if _, duplicate := classRows[source.ClassName]; duplicate {
					t.Fatalf("duplicate captured class: %s", source.ClassName)
				}
				terminal := "System.assert(false, 'P|" + method + "|' + corpusObserved);"
				if strings.Count(source.Source, terminal) != 1 {
					t.Fatalf("%s named source lacks exact terminal observation envelope", tc.ID)
				}
				// Replace only the terminal transport assertion, after its catch.
				// Keep the captured declaration, method and operation unchanged.
				checked := strings.Replace(source.Source, terminal, assertion(method, expected), 1)
				path := filepath.Join(root, "force-app/main/default/classes", source.ClassName+".cls")
				writeFile(t, path, checked)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				classRows[source.ClassName] = tc.ID
				methodNames[tc.ID] = method
				classNames = append(classNames, source.ClassName)
			}
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			// The captured sources reference the required Account/Contact schema.
			// Each shared-runner clone receives its own private record state, ID
			// sequences, transaction state and limits for either execution route.
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			cases := Discover(index, Options{SelectedClasses: classNames})
			if len(cases) != len(data.Cases) {
				t.Fatalf("named discovery: %d methods, want %d", len(cases), len(data.Cases))
			}
			methods, methodErrors := compileTestMethods(cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			byID := make(map[string]TestCase, len(cases))
			for _, tc := range cases {
				id, found := classRows[tc.ClassName]
				_, duplicate := byID[id]
				if !found || duplicate || tc.MethodName != methodNames[id] {
					t.Fatalf("unexpected named case: %#v", tc)
				}
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("%s named compile: %v %v", id, methodErrors[key], programErrors[key])
				}
				byID[id] = tc
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						body := anonymousBody(tc, tc.ExpectedByRouteAPI["anonymous"][api])
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
					t.Run("isTest", func(t *testing.T) {
						named := byID[tc.ID]
						key := testCaseKey(named)
						machine := runner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						machine.SetTestSeeAllData(named.SeeAllData)
						if _, err := machine.ExecuteInClass(programs[key], named.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}
