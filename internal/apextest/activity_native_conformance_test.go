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
func TestActivityNativeConformance(t *testing.T) {
	type namedSource struct {
		ClassName, Source string
	}
	type row struct {
		ID, Stage          string
		CodeByAPI          map[string]string
		ExpectedByRouteAPI map[string]map[string]string
		NamedSourcesByAPI  map[string]namedSource
	}
	var data struct {
		APIVersions       []string `json:"apiVersions"`
		ExpectationStatus string
		MetadataByAPI     map[string]map[string]string
		Cases             []row
	}
	raw, err := os.ReadFile("testdata/conformance/corpus_rt_activity.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.ExpectationStatus != "native captured" || len(data.Cases) != 26 || len(data.MetadataByAPI) != 2 {
		t.Fatalf("native Activity fixture shape: APIs=%v status=%q cases=%d metadata APIs=%d", data.APIVersions, data.ExpectationStatus, len(data.Cases), len(data.MetadataByAPI))
	}
	observations := 0
	for _, api := range data.APIVersions {
		metadata, found := data.MetadataByAPI[api]
		if !found || len(metadata) != 2 || metadata["objects/Activity.object"] == "" {
			t.Fatalf("%s requires the accepted Activity and parent metadata", api)
		}
		for path, source := range metadata {
			if source == "" || filepath.Dir(path) != "objects" || filepath.Ext(path) != ".object" || strings.Contains(path, "..") {
				t.Fatalf("%s invalid captured metadata path: %q", api, path)
			}
			if path != "objects/Activity.object" && !strings.HasSuffix(path, "Parent__c.object") {
				t.Fatalf("%s unexpected captured object: %q", api, path)
			}
		}
	}
	for i, tc := range data.Cases {
		stage := "base"
		if i >= 22 {
			stage = "event"
		} else if i >= 18 {
			stage = "task"
		}
		if tc.ID != fmt.Sprintf("A%03d", i+201) || tc.Stage != stage || len(tc.CodeByAPI) != 2 || len(tc.ExpectedByRouteAPI) != 2 || len(tc.NamedSourcesByAPI) != 2 {
			t.Fatalf("invalid native Activity case: %#v", tc)
		}
		for _, route := range []string{"anonymous", "isTest"} {
			values, found := tc.ExpectedByRouteAPI[route]
			if !found || len(values) != 2 {
				t.Fatalf("%s missing native route: %s", tc.ID, route)
			}
			for _, api := range data.APIVersions {
				expected, found := values[api]
				if !found || (expected == "" && tc.ID != "A208" && tc.ID != "A217") || expected == "?" || strings.HasPrefix(expected, "COMPILE_ERROR") {
					t.Fatalf("%s/%s/%s missing native runtime observation: %q", api, route, tc.ID, expected)
				}
				observations++
			}
		}
		for _, api := range data.APIVersions {
			code, found := tc.CodeByAPI[api]
			if !found || code == "" {
				t.Fatalf("%s/%s missing captured operation", api, tc.ID)
			}
			source, found := tc.NamedSourcesByAPI[api]
			if !found || source.ClassName == "" || source.Source == "" ||
				!strings.Contains(source.Source, "@IsTest private class "+source.ClassName+" {") ||
				!strings.Contains(source.Source, "@IsTest static void "+tc.ID+"() {") ||
				strings.Count(source.Source, "\n"+code+"\n") != 1 {
				t.Fatalf("%s/%s missing source-bound named observation", api, tc.ID)
			}
		}
	}
	if observations != 104 {
		t.Fatalf("native Activity matrix: %d observations, want 104", observations)
	}
	assertion := func(id, expected string) string {
		return "String expectedText=" + conformanceApexString(expected) + "; String observedText=corpusObserved; " +
			"System.assert(expectedText.equals(observedText)," + conformanceApexString(id) + "+' expected <'+expectedText+'> actual <'+observedText+'>');"
	}
	anonymousBody := func(tc row, api, expected string) string {
		return "String corpusObserved; try { Object r;\n" + tc.CodeByAPI[api] +
			"\ncorpusObserved=String.valueOf(r); } catch(Exception e) { corpusObserved='EXC|'+e.getTypeName()+'|'+e.getMessage(); }\n" + assertion(tc.ID, expected)
	}
	expectationsChecked := 0
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			// M002/M003 rejected Task/Event redeclarations. Only the accepted
			// base Activity and parent XML is installed for all runtime rows.
			for path, source := range data.MetadataByAPI[api] {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), source)
			}
			classRows := make(map[string]string, len(data.Cases))
			methodNames := make(map[string]string, len(data.Cases))
			var classNames []string
			for _, tc := range data.Cases {
				source, method, expected := tc.NamedSourcesByAPI[api], tc.ID, tc.ExpectedByRouteAPI["isTest"][api]
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
			// The captured sources reference the accepted Activity/parent schema.
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
						body := anonymousBody(tc, api, tc.ExpectedByRouteAPI["anonymous"][api])
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
						expectationsChecked++
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
						expectationsChecked++
					})
				})
			}
		})
	}
	if expectationsChecked != 104 {
		t.Fatalf("checked %d native Activity expectations, want 104", expectationsChecked)
	}
}
