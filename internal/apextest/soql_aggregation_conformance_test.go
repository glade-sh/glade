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

// Captured Apex and exact native text at both source API endpoints. No org,
// network, credentials or probe tooling participates in the CI execution.
func TestSOQLAggregationOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected      string
		Compile, RuntimeCompile bool
		NativeNull              bool
		NativeLine              int
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Prelude      string
		Cases        []familyCase
		Controls     []familyCase
		Remaining    []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/soql_aggregation.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 236 || len(data.Controls) != 36 || len(data.Remaining) != 3 {
		t.Fatalf("oracle rows: %d family + %d controls + %d carried", len(data.Cases), len(data.Controls), len(data.Remaining))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, tc := range rows {
		if tc.ID == "" || seen[tc.ID] || tc.Expected == "?" {
			t.Fatalf("invalid/repeated asserted row: %#v", tc)
		}
		seen[tc.ID] = true
		if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && tc.NativeLine == 0 {
			t.Fatalf("missing native diagnostic location: %s", tc.ID)
		}
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Reason == "" || strings.HasPrefix(row.Owner, "SOQL aggregation") {
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
	caseCode := func(tc familyCase) string {
		if tc.Compile {
			return tc.Code
		}
		return "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if !tc.Compile {
			prefix += data.Prelude + "\n"
		}
		return prefix + caseCode(tc)
	}
	rejectedSource := func(tc familyCase, named bool) string {
		prefix := ""
		if named {
			prefix = "@IsTest private class SOQLAggregationCompile" + tc.ID + " {\n@IsTest static void observed(){\n"
		}
		prefix += "P pq=new P();\n"
		if tc.RuntimeCompile {
			prefix += data.Prelude + "\n"
		}
		line := strings.Count(prefix, "\n") + 1
		if tc.NativeLine < line {
			t.Fatalf("native line %d precedes rejection source for %s", tc.NativeLine, tc.ID)
		}
		prefix += strings.Repeat("\n", tc.NativeLine-line) + caseCode(tc)
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
			// The captured compiler payload is limited to 300 characters.
			if len(observed) > 300 {
				observed = observed[:300]
			}
			return "COMPILE_ERROR\t" + observed
		}
		return "ACCEPTED"
	}
	checkExecution := func(t *testing.T, tc familyCase, err error) {
		t.Helper()
		if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
			observed := "UNCAUGHT|"
			if err != nil {
				observed += err.Error()
			}
			if observed != tc.Expected {
				t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "SOQL aggregation", api, "")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			// Keep the standard schema authoritative instead of inferring fields
			// from deliberately rejected SOQL/property rows.
			metadata := gladeschema.Schema{Objects: []gladeschema.Object{
				{Name: "Account", Fields: []gladeschema.Field{{Name: "Id", Type: "Id"}}},
				{Name: "Contact", Fields: []gladeschema.Field{{Name: "Id", Type: "Id"}}},
			}}
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
				return typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, metadata)
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
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class SOQLAggregationProbe {\n")
			executable := 0
			for _, tc := range rows {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "SOQLAggregationProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
			namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"SOQLAggregationProbe"}})
			if len(namedCases) != executable {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), executable)
			}
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			namedRows := map[string]TestCase{}
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
					counts.Run(t, "anonymous", tc.ID, "anonymous", func(t *testing.T) {
						source := body(tc)
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							source = rejectedSource(tc, false)
						}
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
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
						_, err = anonymousRunner.execute(program)
						checkExecution(t, tc, err)
					})
					counts.Run(t, "isTest", tc.ID, "isTest", func(t *testing.T) {
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							path := filepath.Join(root, "SOQLAggregationCompile"+tc.ID+".cls")
							writeFile(t, path, rejectedSource(tc, true))
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							if observed := compileText(sema.Analyze(buildIndex(path))); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
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
						_, err := machine.ExecuteInClass(programs[key], row.ClassName)
						checkExecution(t, tc, err)
					})
				})
			}
		})
	}
}
