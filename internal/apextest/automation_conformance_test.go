package apextest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// A43 native requests, local callbacks and rejection diagnostics at API 62/67.
// Hosted execution is bounded by explicit local errors. CI loads only this export.
func TestAutomationOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
		NativeLine          int
		LocalBoundaryText   string
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Prelude      string
		Files        map[string]string
		Schema       gladeschema.Schema `json:"schema"`
		Cases        []familyCase
		Remaining    []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/automation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 304 || len(data.Remaining) != 0 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle: %d asserted + %d carried at %v", len(data.Cases), len(data.Remaining), data.APIVersions)
	}
	seen := map[string]bool{}
	for _, tc := range data.Cases {
		if tc.ID == "" || seen[tc.ID] || tc.Expected == "?" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && tc.NativeLine == 0 {
			t.Fatalf("invalid asserted row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A43" || row.Reason == "" {
			t.Fatalf("invalid carried row: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
	}
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
  if(expectedNull){
   System.assert(v==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  if(expectedText=='null'){System.assert(v!=null,id+' expected String null, actual raw null');}
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		if tc.LocalBoundaryText != "" {
			return "Object r; " + tc.Code
		}
		return prefix + data.Prelude + "\ntry {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	rejectedSource := func(tc familyCase, named bool) string {
		prefix := ""
		if named {
			prefix = "@IsTest private class AutomationCompile" + tc.ID + " {\n@IsTest static void observed(){\n"
		}
		prefix += "P pq=new P(); " + data.Prelude + "\n"
		if !tc.Compile {
			prefix += "Object r;\n"
		}
		line := strings.Count(prefix, "\n") + 1
		if tc.NativeLine < line {
			t.Fatalf("native line %d precedes rejection body for %s", tc.NativeLine, tc.ID)
		}
		prefix += strings.Repeat("\n", tc.NativeLine-line) + tc.Code
		if named {
			prefix += "\n}\n}\n"
		}
		return prefix
	}
	compileText := func(result sema.Result) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			observed := fmt.Sprintf("line %d: %s", line, strings.ReplaceAll(item.Message, "\n", " "))
			if len(observed) > 300 {
				observed = observed[:300]
			}
			return "COMPILE_ERROR\t" + observed
		}
		return "ACCEPTED"
	}
	assertBoundary := func(t *testing.T, expected string, err error) {
		t.Helper()
		var local *vm.RuntimeError
		if !errors.As(err, &local) || local.Type != "UnsupportedFeature" || local.Message != expected {
			t.Fatalf("local boundary expected <%s> actual <%v>", expected, err)
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "automation", api)
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, source := range data.Files {
				writeFile(t, filepath.Join(root, name), source)
			}
			names := []string{"P"}
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
				names = append(names, name)
			}
			sort.Strings(names)
			paths := make([]string, 0, len(names))
			for _, name := range names {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				files := append([]string{}, paths...)
				if extra != "" {
					files = append(files, extra)
				}
				return typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, data.Schema)
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("helper semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			// Compile and link each API's named matrix once; runCase preserves
			// ordinary @IsTest isolation while reusing the shared linked base.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class AutomationProbe {\n")
			executable := 0
			for _, tc := range data.Cases {
				if tc.NativeLine > 0 {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "AutomationProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedCases := Discover(namedIndex, Options{})
			if len(namedCases) != executable {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), executable)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			runtimeMethods := indexTestRuntimeMethods(methods)
			counters := newRunPerfCounters(false)
			namedByID := make(map[string]TestCase, len(namedCases))
			for _, tc := range namedCases {
				id := strings.TrimPrefix(tc.MethodName, "observed")
				if tc.ClassName != "AutomationProbe" || namedByID[id].MethodName != "" {
					t.Fatalf("unexpected/repeated named row: %#v", tc)
				}
				namedByID[id] = tc
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					kind := "exact"
					if tc.LocalBoundaryText != "" {
						kind = ""
					}
					counts.run(t, "anonymous", "anonymous", kind, func(t *testing.T) {
						source := body(tc)
						if tc.NativeLine > 0 {
							source = rejectedSource(tc, false)
						}
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if tc.NativeLine > 0 {
							if observed := compileText(analysis); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						// The probe enables tracing; named cases also cover the
						// untraced path. Trace conversion must preserve DML errors.
						_, executionErr := anonymousRunner.execute(program, func(machine *vm.VM) { machine.SetTraceEnabled(true) })
						if tc.LocalBoundaryText != "" {
							assertBoundary(t, tc.LocalBoundaryText, executionErr)
						} else if executionErr != nil {
							t.Fatal(executionErr)
						}
					})
					counts.run(t, "isTest", "@IsTest", kind, func(t *testing.T) {
						if tc.NativeLine > 0 {
							path := filepath.Join(root, "AutomationCompile"+tc.ID+".cls")
							writeFile(t, path, rejectedSource(tc, true))
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							if observed := compileText(sema.Analyze(buildIndex(path))); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
						row := namedByID[tc.ID]
						key := testCaseKey(row)
						if tc.LocalBoundaryText != "" {
							if methodErrors[key] != nil || programErrors[key] != nil {
								t.Fatalf("boundary compile: %v %v", methodErrors[key], programErrors[key])
							}
							machine := namedRunner.newMachine()
							machine.EnableTestContext()
							if err := machine.RegisterMethod(methods[key]); err != nil {
								t.Fatal(err)
							}
							_, executionErr := machine.ExecuteInClass(programs[key], row.ClassName)
							assertBoundary(t, tc.LocalBoundaryText, executionErr)
							return
						}
						seed := namedRunner.org.CloneRuntimeOrg()
						initializeTestOrg(&seed)
						result := runCase(context.Background(), row, methods[key], runtimeMethods[testMethodSourceKey(row.ClassName, row.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
						if result.Status != testreport.StatusPass {
							t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
						}
					})
				})
			}
		})
	}
}
