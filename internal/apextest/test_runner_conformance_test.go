package apextest

import (
	"context"
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
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native test-only observations use real setup/method isolation, not an
// anonymous VM with test context toggled on. CI needs only the owned export.
func TestTestRunnerOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected, Class string
		CodeByAPI                 map[string]string
		Compile, NativeNull       bool
		NativeLine                int
	}
	var data struct {
		APIVersions                                     []string `json:"apiVersions"`
		Declarations, TestHeads                         map[string]string
		Resources, NamedControls                        map[string]string
		MetadataFiles                                   map[string]string
		Cases, NativeCases, Controls, AnonymousControls []familyCase
		CompileControls                                 []familyCase
		Remaining                                       []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/test_runner.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 36 || len(data.NativeCases) != 206 || len(data.Controls) != 161 || len(data.AnonymousControls) != 13 || len(data.CompileControls) != 3 || len(data.NamedControls) != 25 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle: %d anonymous/compiler, %d native, %d controls, %d anonymous controls, %d named counterparts at %v", len(data.Cases), len(data.NativeCases), len(data.Controls), len(data.AnonymousControls), len(data.NamedControls), data.APIVersions)
	}
	seen := map[string]bool{}
	for _, tc := range append(append(append(append([]familyCase{}, data.Cases...), data.NativeCases...), data.Controls...), append(data.AnonymousControls, data.CompileControls...)...) {
		if tc.ID == "" || seen[tc.ID] || tc.Expected == "?" || tc.Compile && tc.NativeLine == 0 {
			t.Fatalf("invalid oracle row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A30" || row.Reason == "" {
			t.Fatalf("invalid carry: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
	}
	body := func(tc familyCase) string {
		return "Object r; Boolean observedNull=false; String observedText;\ntry {" + tc.Code +
			" observedNull=(r==null); observedText=''+String.valueOf(r); } catch(Exception e) { observedText='EXC|'+e.getTypeName()+'|'+e.getMessage(); }\n" +
			"observedText=observedText.replace('\\r','\\\\r').replace('\\n','\\\\n');\nString expectedText=" + conformanceApexString(tc.Expected) + ";\n" +
			func() string {
				if tc.NativeNull {
					return "System.assert(observedNull," + conformanceApexString(tc.ID) + "+' expected raw null actual <'+observedText+'>');\n" +
						"System.assert(expectedText.equals(observedText)," + conformanceApexString(tc.ID) + "+' expected <'+expectedText+'> actual <'+observedText+'>');"
				}
				return "System.assert(expectedText.equals(observedText)," + conformanceApexString(tc.ID) + "+' expected <'+expectedText+'> actual <'+observedText+'>');"
			}()
	}
	compileText := func(result sema.Result) string {
		for _, d := range result.Diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			message := d.Message
			if d.NativeMessage != "" {
				message = d.NativeMessage
			}
			line := 0
			if d.Range != nil {
				line = d.Range.Start.Line
			}
			text := fmt.Sprintf("line %d: %s", line, strings.ReplaceAll(message, "\n", " "))
			if len(text) > 300 {
				text = text[:300]
			}
			return "COMPILE_ERROR\t" + text
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "test runner", api, "")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			classDir := filepath.Join(root, "force-app/main/default/classes")
			writeClass := func(name, source string) string {
				path := filepath.Join(classDir, name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				return path
			}
			names := make([]string, 0, len(data.Declarations))
			for name := range data.Declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			var fixtureFiles []string
			for _, name := range names {
				fixtureFiles = append(fixtureFiles, writeClass(name, data.Declarations[name]))
			}
			// Compiler rows retain their captured pq call without changing the
			// row text. It is never executed on a rejected compilation.
			fixtureFiles = append(fixtureFiles, writeClass("P", "public class P {public void out(String id,Object value){}}"))
			for name, content := range data.Resources {
				path := filepath.Join(root, "force-app/main/default/staticresources", name+".resource")
				writeFile(t, path, content)
				writeFile(t, path+"-meta.xml", "<StaticResource><cacheControl>Private</cacheControl><contentType>text/csv</contentType></StaticResource>")
			}
			for path, content := range data.MetadataFiles {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), content)
			}
			baseIndex := loadTestIndex(t, root)
			if analysis := sema.Analyze(baseIndex); analysis.HasErrors() {
				t.Fatalf("fixture semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			anonymousRunner := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			nativeRows := append(append([]familyCase{}, data.NativeCases...), data.Controls...)
			for i := range nativeRows {
				if nativeRows[i].CodeByAPI != nil {
					code, ok := nativeRows[i].CodeByAPI[api]
					if !ok || code == "" {
						t.Fatalf("missing captured source for %s at API %s", nativeRows[i].ID, api)
					}
					nativeRows[i].Code = code
				}
			}
			byID := make(map[string]familyCase, len(nativeRows))
			for _, tc := range nativeRows {
				byID[tc.ID] = tc
			}
			classNames := make([]string, 0, len(data.TestHeads))
			for name := range data.TestHeads {
				classNames = append(classNames, name)
			}
			sort.Strings(classNames)
			for _, name := range classNames {
				var source strings.Builder
				source.WriteString(data.TestHeads[name])
				for _, tc := range nativeRows {
					if tc.Class == name {
						fmt.Fprintf(&source, " @IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
					}
				}
				source.WriteString("}\n")
				writeClass(name, source.String())
			}
			namedIndex := loadTestIndex(t, root)
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
			namedCases := Discover(namedIndex, Options{SelectedClasses: classNames})
			if len(namedCases) != len(nativeRows) {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), len(nativeRows))
			}
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			runtimeMethods := indexTestRuntimeMethods(methods)
			setups, setupErrors, setupPrograms, setupProgramErrors := compileTestSetupMethodsForClasses(namedIndex, testCaseClassSet(namedCases), newSourceCache())
			// Match Run's setup registration before cloning an isolated method.
			if err := registerTestRuntime(namedRunner.base, flattenSetupMethods(setups)); err != nil {
				t.Fatal(err)
			}
			namedRunner.base.FreezeClassLookup()
			counters := newRunPerfCounters(false)
			seeds := map[string]testSetupResult{}
			for _, name := range classNames {
				var limits vm.Limits
				seed, random, err, shared := prepareTestSetupOrg(context.Background(), name, namedRunner.base, nil, setups[name], setupErrors[name], setupPrograms[name], setupProgramErrors[name], nil, namedRunner.newOrg(), Options{Parallelism: 1}, counters, &limits)
				if err != nil {
					t.Fatalf("%s setup: %v", name, err)
				}
				seeds[name] = testSetupResult{Org: seed, Random: random, Limits: limits, OrgIsShared: shared}
			}
			byMethod := map[string]TestCase{}
			for _, row := range namedCases {
				id := strings.TrimPrefix(row.MethodName, "observed")
				if _, duplicate := byMethod[id]; duplicate {
					t.Fatalf("duplicate native row: %s", id)
				}
				byMethod[id] = row
			}
			runNamed := func(t *testing.T, tc familyCase) {
				t.Helper()
				row, ok := byMethod[tc.ID]
				if !ok {
					t.Fatalf("missing named row %s", tc.ID)
				}
				key := testCaseKey(row)
				seed := seeds[row.ClassName]
				result := runCase(context.Background(), row, methods[key], runtimeMethods[testMethodSourceKey(row.ClassName, row.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed.Org, seed.Random, Options{NoDiskCache: true, Parallelism: 1}, true, nil, counters, seed.Limits)
				if !strings.HasPrefix(tc.Expected, "TEST_Fail|") {
					if result.Status != testreport.StatusPass {
						t.Fatalf("expected <%s>; named status %s problem %v", tc.Expected, result.Status, result.Problem)
					}
					return
				}
				observed := ""
				if result.Problem != nil {
					typeName := result.Problem.Type
					if !strings.Contains(typeName, ".") {
						typeName = "System." + typeName
					}
					observed = "TEST_Fail|" + typeName + ": " + result.Problem.Message
					observed = strings.ReplaceAll(strings.ReplaceAll(observed, "\r", "\\r"), "\n", "\\n")
				}
				if result.Status != testreport.StatusFail || observed != tc.Expected {
					t.Fatalf("expected <%s> actual <%s>; status %s", tc.Expected, observed, result.Status)
				}
			}
			for _, tc := range append(append(append([]familyCase{}, data.Cases...), data.CompileControls...), data.AnonymousControls...) {
				t.Run(tc.ID, func(t *testing.T) {
					if tc.Compile {
						// Inline class declarations use the same anonymous admission
						// gate before either execution route (as in A38). A root
						// @TestSetup method likewise stays an anonymous declaration.
						parsed := apexast.NewParser().ParseSource("A30Compile.cls", tc.Code)
						declRoot := t.TempDir()
						var declPaths []string
						for _, decl := range parsed.Declarations {
							if decl.Kind != apexast.DeclarationClass && decl.Kind != apexast.DeclarationInterface && decl.Kind != apexast.DeclarationEnum {
								continue
							}
							source := strings.Repeat("\n", tc.NativeLine-1) + tc.Code[decl.Range.Start.Offset:decl.Range.End.Offset]
							path := filepath.Join(declRoot, decl.Name+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							declPaths = append(declPaths, path)
						}
						routes := []bool{false, true}
						// Inline declarations and root @TestSetup rows have one captured
						// admission route; do not label repeated analysis as named execution.
						if len(declPaths) > 0 || strings.HasPrefix(tc.Code, "@TestSetup") {
							routes = []bool{false}
						}
						for _, named := range routes {
							route := "anonymous"
							if named {
								route = "isTest"
							}
							counts.Run(t, route, tc.ID, route, func(t *testing.T) {
								var analysis sema.Result
								if len(declPaths) > 0 {
									transient := typesys.Build(project.Project{Root: declRoot, ApexFiles: declPaths, SourceAPIVersion: api}, gladeschema.Schema{})
									if transient.HasErrors() {
										t.Fatalf("declaration parser: %v", transient.Diagnostics)
									}
									merged := baseIndex
									merged.Types = append(append([]typesys.TypeSymbol(nil), baseIndex.Types...), transient.Types...)
									analysis = sema.AnalyzeAnonymousDeclarationsInContext(merged, transient)
								} else {
									prefix := "P pq=new P();\n"
									if named && !strings.HasPrefix(tc.Code, "@TestSetup") {
										prefix = "@IsTest private class A30Compiler" + tc.ID + " {\n @IsTest static void observed(){\n" + prefix
									}
									source := prefix + strings.Repeat("\n", tc.NativeLine-strings.Count(prefix, "\n")-1) + tc.Code
									if named && !strings.HasPrefix(tc.Code, "@TestSetup") {
										path := filepath.Join(declRoot, "A30Compiler"+tc.ID+".cls")
										writeFile(t, path, source+"\n}\n}\n")
										writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
										files := append(append([]string{}, fixtureFiles...), path)
										analysis = sema.Analyze(typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, gladeschema.Schema{}))
									} else {
										analysis = sema.AnalyzeAnonymous(baseIndex, source, api)
									}
								}
								if observed := compileText(analysis); observed != tc.Expected {
									t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
								}
							})
						}
						return
					}
					counts.Run(t, "anonymous", tc.ID, "anonymous", func(t *testing.T) {
						source := body(tc)
						if analysis := sema.AnalyzeAnonymous(baseIndex, source, api); analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := anonymousRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					if counterpartID, ok := data.NamedControls[tc.ID]; ok {
						counts.Run(t, "isTest", counterpartID, "isTest", func(t *testing.T) {
							counterpart := byID[counterpartID]
							if counterpart.Code != tc.Code || counterpart.ID == "" {
								t.Fatalf("missing exact native named counterpart for %s", tc.ID)
							}
							runNamed(t, counterpart)
						})
					} else if len(tc.ID) > 0 && tc.ID[0] == 'R' {
						t.Fatalf("missing native named counterpart for %s", tc.ID)
					}
				})
			}
			for _, tc := range nativeRows {
				t.Run(tc.ID, func(t *testing.T) {
					counts.Run(t, "isTest", tc.ID, "isTest", func(t *testing.T) { runNamed(t, tc) })
				})
			}
		})
	}
}
