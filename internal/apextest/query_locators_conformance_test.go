package apextest

import (
	"encoding/json"
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
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned API 62/67 observations, including controls for lazy iterator accounting.
// Each route uses a clone of one linked API runner, with fresh seeded records.
func TestQueryLocatorsOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
		DiagnosticLine      int
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Prelude      string
		Cases        []familyCase
		Remaining    []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/query_locators.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 289 || len(data.Remaining) != 23 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle matrix: %d asserted, %d carried, APIs %v", len(data.Cases), len(data.Remaining), data.APIVersions)
	}
	seen := map[string]bool{}
	for _, row := range data.Cases {
		if seen[row.ID] || row.ID == "" || row.Expected == "?" {
			t.Fatalf("invalid/repeated asserted row: %#v", row)
		}
		seen[row.ID] = true
	}
	for _, row := range data.Remaining {
		if seen[row.ID] || row.ID == "" || row.Owner == "" || row.Owner == "query locators" || row.Reason == "" {
			t.Fatalf("invalid/repeated carried row: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
	}
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  if(expectedNull){
   System.assert(v==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	rowCode := func(row familyCase) string {
		if row.Compile {
			return row.Code
		}
		return "try {Object r; " + row.Code + " pq.out('" + row.ID + "',r);}catch(Exception e){pq.err('" + row.ID + "',e);}"
	}
	body := func(row familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(row.Expected) + ";\n"
		if row.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if row.Compile {
			return prefix + rowCode(row)
		}
		return prefix + data.Prelude + "\n" + rowCode(row)
	}
	compileText := func(diagnostics []diagnostic.Diagnostic) string {
		for _, d := range diagnostics {
			if d.Severity == diagnostic.Error {
				line := 0
				if d.Range != nil {
					line = d.Range.Start.Line
				}
				return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, d.Message)
			}
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "query locators", api)
			root := t.TempDir()
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
			}
			names := make([]string, 0, len(declarations))
			for name := range declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			paths := make([]string, 0, len(names))
			for _, name := range names {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				files := append([]string{}, paths...)
				if extra != "" {
					files = append(files, extra)
				}
				return typesys.Build(project.Project{Root: root, ApexFiles: files}, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})

			// Compile the @IsTest matrix and its invocations once for this API.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class QueryLocatorsProbe {\n")
			executable := 0
			for _, row := range data.Cases {
				if strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "QueryLocatorsProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatal(namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
			namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"QueryLocatorsProbe"}})
			if len(namedCases) != executable {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), executable)
			}
			namedMethods, methodErrors := compileTestMethods(namedCases)
			namedPrograms, programErrors := compileTestInvokePrograms(namedCases)
			byID := make(map[string]TestCase, len(namedCases))
			for _, tc := range namedCases {
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compilation %s: %v %v", tc.MethodName, methodErrors[key], programErrors[key])
				}
				byID[strings.TrimPrefix(tc.MethodName, "observed")] = tc
			}
			for _, row := range data.Cases {
				t.Run(row.ID, func(t *testing.T) {
					for _, route := range []string{"anonymous", "isTest"} {
						countRoute := route
						if route == "isTest" {
							countRoute = "@IsTest"
						}
						counts.run(t, route, countRoute, "exact", func(t *testing.T) {
							if strings.HasPrefix(row.Expected, "COMPILE_ERROR\t") {
								// Put the intact row at the native capture's line, preserving
								// both diagnostic text and line number on each route.
								var diagnostics []diagnostic.Diagnostic
								if route == "anonymous" {
									source := "P pq=new P(); QL helper=new QL();" + strings.Repeat("\n", row.DiagnosticLine-1) + rowCode(row)
									diagnostics = sema.AnalyzeAnonymous(index, source, api).Diagnostics
								} else {
									path := filepath.Join(root, "QueryLocatorsRejected.cls")
									source := "@IsTest private class QueryLocatorsRejected {\n@IsTest static void observed(){\nP pq=new P(); QL helper=new QL();" + strings.Repeat("\n", row.DiagnosticLine-3) + rowCode(row) + "\n}}"
									writeFile(t, path, source)
									writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
									rejectedIndex := buildIndex(path)
									diagnostics = append(rejectedIndex.Diagnostics, sema.Analyze(rejectedIndex).Diagnostics...)
								}
								if observed := compileText(diagnostics); observed != row.Expected {
									t.Fatalf("expected <%s> actual <%s>", row.Expected, observed)
								}
								return
							}
							if route == "isTest" {
								tc, ok := byID[row.ID]
								if !ok {
									t.Fatal("missing named row")
								}
								key := testCaseKey(tc)
								machine := namedRunner.newMachine()
								machine.EnableTestContext()
								if err := machine.RegisterMethod(namedMethods[key]); err != nil {
									t.Fatal(err)
								}
								if _, err := machine.ExecuteInClass(namedPrograms[key], tc.ClassName); err != nil {
									t.Fatal(err)
								}
								return
							}
							source := body(row)
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
								t.Fatal(analysis.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
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
		})
	}
}
