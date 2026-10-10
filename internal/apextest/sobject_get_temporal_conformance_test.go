package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Every API/route/case expectation comes from that route's native capture.
func TestSObjectGetTemporalOrgConformance(t *testing.T) {
	var data struct {
		APIVersions                []string `json:"apiVersions"`
		ExpectationStatus          string   `json:"expectationStatus"`
		Locale, Timezone, Language string
		Cases                      []struct{ ID, Code string }
		NativeObservations         []struct {
			APIVersion          string `json:"apiVersion"`
			Route, ID, Observed string
		} `json:"nativeObservations"`
	}
	raw, err := os.ReadFile("testdata/conformance/corpus_rt_sobject_get.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 22 || strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.ExpectationStatus != "native captured" {
		t.Fatalf("oracle cases=%d versions=%v status=%q", len(data.Cases), data.APIVersions, data.ExpectationStatus)
	}
	if data.Locale == "" || data.Timezone == "" || data.Language == "" {
		t.Fatal("missing native capture user context")
	}
	seen := map[string]bool{}
	for i, row := range data.Cases {
		if row.ID != fmt.Sprintf("R%03d", i+1) || seen[row.ID] || row.Code == "" {
			t.Fatalf("invalid oracle row: %#v", row)
		}
		seen[row.ID] = true
	}
	native := map[string]string{}
	for _, row := range data.NativeObservations {
		key := row.APIVersion + "/" + row.Route + "/" + row.ID
		if _, exists := native[key]; exists || !seen[row.ID] ||
			(row.APIVersion != "62.0" && row.APIVersion != "67.0") ||
			(row.Route != "anonymous" && row.Route != "isTest") ||
			row.Observed == "" || row.Observed == "?" || row.Observed == "MISSING" || strings.HasPrefix(row.Observed, "COMPILE_ERROR") {
			t.Fatalf("invalid native observation: %#v", row)
		}
		native[key] = row.Observed
	}
	if len(native) != 88 {
		t.Fatalf("native matrix has %d/88 observations", len(native))
	}
	for _, api := range data.APIVersions {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, row := range data.Cases {
				if _, ok := native[api+"/"+route+"/"+row.ID]; !ok {
					t.Fatalf("missing native observation: %s/%s/%s", api, route, row.ID)
				}
			}
		}
	}
	body := func(id, code, expected string) string {
		// Both native transports concatenate the result into a tagged String.
		// Preserve that conversion before comparing, including a null result.
		tag := "P|" + id + "|"
		return "String corpusObserved; try { Object r; " + code +
			" corpusObserved=String.valueOf(r); } catch(Exception e) { " +
			"corpusObserved='EXC|'+e.getTypeName()+'|'+e.getMessage(); } " +
			"corpusObserved=" + conformanceApexString(tag) + "+corpusObserved; " +
			"String expectedText=" + conformanceApexString(tag+expected) + "; " +
			"System.assert(expectedText.equals(corpusObserved)," + conformanceApexString(id+" expected <") +
			"+expectedText+'> actual <'+corpusObserved+'>');"
	}
	configure := func(machine *vm.VM) {
		// Match the captured user's local Datetime parsing without runAs.
		machine.SetCurrentUser(storage.Record{ID: "005-temporal-user", Object: "User", Fields: map[string]storage.Value{
			"LocaleSidKey": storage.StringValue(data.Locale), "TimeZoneSidKey": storage.StringValue(data.Timezone), "LanguageLocaleKey": storage.StringValue(data.Language),
		}})
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			anonymous := newConformanceRunner(t, typesys.Index{}, conformanceRunnerOptions{})
			root := t.TempDir()
			const className = "SObjectGetTemporalOracle"
			var source strings.Builder
			source.WriteString("@IsTest private class " + className + " {\n")
			for _, row := range data.Cases {
				fmt.Fprintf(&source, "@IsTest static void %s(){%s}\n", row.ID, body(row.ID, row.Code, native[api+"/isTest/"+row.ID]))
			}
			source.WriteString("}\n")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			path := filepath.Join(root, "force-app/main/default/classes", className+".cls")
			writeFile(t, path, source.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			tests := Discover(index, Options{SelectedClasses: []string{className}})
			if len(tests) != len(data.Cases) {
				t.Fatalf("named discovery returned %d/%d rows", len(tests), len(data.Cases))
			}
			methods, methodErrors := compileTestMethods(tests)
			programs, programErrors := compileTestInvokePrograms(tests)
			byID := map[string]TestCase{}
			for _, test := range tests {
				if _, exists := byID[test.MethodName]; exists || test.ClassName != className || !seen[test.MethodName] {
					t.Fatalf("unexpected named method: %#v", test)
				}
				key := testCaseKey(test)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named lowering %s: %v %v", key, methodErrors[key], programErrors[key])
				}
				byID[test.MethodName] = test
			}
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						source := body(row.ID, row.Code, native[api+"/anonymous/"+row.ID])
						if analysis := sema.AnalyzeAnonymous(typesys.Index{}, source, api); analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := anonymous.execute(program, configure); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						test := byID[row.ID]
						key := testCaseKey(test)
						machine := namedRunner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						configure(machine)
						// Test context snapshots the configured execution user.
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(programs[key], test.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}
