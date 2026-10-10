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

// Fixed Salesforce observations and local service boundaries need no org or
// probe project in CI. Local boundary checks are separate from native answers.
func TestReportsOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected, Basis, Owner, Carry string
		Compile, NativeNull                     bool
	}
	var data struct {
		APIVersions                      []string `json:"apiVersions"`
		Declarations                     map[string]string
		Cases, Controls, LocalBoundaries []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/reports.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 281 || len(data.Controls) != 140 || len(data.LocalBoundaries) != 9 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle shape: rows=%d controls=%d local boundaries=%d versions=%v", len(data.Cases), len(data.Controls), len(data.LocalBoundaries), data.APIVersions)
	}
	rows := append(append(append([]familyCase{}, data.Cases...), data.Controls...), data.LocalBoundaries...)
	seen := make(map[string]bool)
	for _, tc := range rows {
		if tc.ID == "" || seen[tc.ID] || tc.Code == "" || tc.Expected == "?" || tc.Carry != "" && tc.Owner == "" {
			t.Fatalf("invalid/repeated oracle row: %#v", tc)
		}
		seen[tc.ID] = true
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
  if(expectedText=='null'){System.assert(v!=null,id+' expected String null, actual raw null');}
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
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
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	rejectedSource := func(tc familyCase, named bool) string {
		line := 0
		if _, err := fmt.Sscanf(tc.Expected, "COMPILE_ERROR\tline %d:", &line); err != nil || line < 3 {
			t.Fatalf("missing native diagnostic line for %s: %s", tc.ID, tc.Expected)
		}
		prefix := "P pq=new P();\n"
		if named {
			prefix = "@IsTest private class ReportsRejected" + tc.ID + " {\n@IsTest static void observed(){\n" + prefix
		}
		prefix += strings.Repeat("\n", line-strings.Count(prefix, "\n")-1)
		prefix += tc.Code
		if named {
			prefix += "\n}\n}\n"
		}
		return prefix
	}
	compileText := func(result sema.Result) string {
		for _, d := range result.Diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			line := 0
			if d.Range != nil {
				line = d.Range.Start.Line
			}
			return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, strings.ReplaceAll(d.Message, "\n", " "))
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "reports", api, "")
			root := t.TempDir()
			names := []string{"P"}
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				names = append(names, name)
				declarations[name] = source
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
				return typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("helper semantics: %v", analysis.Diagnostics)
			}
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class ReportsProbe {\n")
			executable := 0
			for _, tc := range rows {
				if tc.Carry != "" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "ReportsProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true})
			namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"ReportsProbe"}})
			if len(namedCases) != executable {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), executable)
			}
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			namedRows := make(map[string]TestCase)
			for _, tc := range namedCases {
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compilation %s: %v %v", tc.MethodName, methodErrors[key], programErrors[key])
				}
				id := strings.TrimPrefix(tc.MethodName, "observed")
				if _, exists := namedRows[id]; exists {
					t.Fatalf("repeated named row: %s", id)
				}
				namedRows[id] = tc
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					if tc.Carry != "" {
						t.Logf("%s carried to %s: %s; native <%s>", tc.ID, tc.Owner, tc.Carry, tc.Expected)
						return
					}
					counts.Run(t, "anonymous", tc.ID, "anonymous", func(t *testing.T) {
						source := body(tc)
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							source = rejectedSource(tc, false)
						}
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							if observed := compileText(analysis); observed != tc.Expected {
								t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.Expected, observed)
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
						if _, err := anonymousRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					counts.Run(t, "isTest", tc.ID, "isTest", func(t *testing.T) {
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							path := filepath.Join(root, "ReportsRejected"+tc.ID+".cls")
							writeFile(t, path, rejectedSource(tc, true))
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							if observed := compileText(sema.Analyze(buildIndex(path))); observed != tc.Expected {
								t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.Expected, observed)
							}
							return
						}
						row, exists := namedRows[tc.ID]
						if !exists {
							t.Fatalf("missing named row: %s", tc.ID)
						}
						key := testCaseKey(row)
						machine := namedRunner.newMachine()
						if err := machine.RegisterMethod(methods[key]); err != nil {
							t.Fatal(err)
						}
						machine.EnableTestContext()
						if _, err := machine.ExecuteInClass(programs[key], row.ClassName); err != nil {
							t.Fatal(err)
						}
					})
				})
			}
		})
	}
}
