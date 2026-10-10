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
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned Salesforce observations at both API endpoints. CI uses local inputs.
func TestSOQLSelectionOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Prelude      string
		Cases        []familyCase
		Remaining    []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/soql_selection.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 268 || len(data.Remaining) != 17 {
		t.Fatalf("oracle rows: %d exact-text cases + %d unasserted", len(data.Cases), len(data.Remaining))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	seen := map[string]bool{}
	for _, tc := range data.Cases {
		if seen[tc.ID] || tc.ID == "" || tc.Expected == "?" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && !tc.Compile {
			t.Fatalf("invalid/repeated asserted row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	for _, row := range data.Remaining {
		if seen[row.ID] || row.ID == "" || row.Owner == "" || row.Reason == "" {
			t.Fatalf("invalid/repeated unasserted row: %#v", row)
		}
		seen[row.ID] = true
		if row.Owner == "SOQL selection" {
			t.Logf("unfinished diagnostic %s owner=%s: %s", row.ID, row.Owner, row.Reason)
		} else {
			t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
		}
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
		if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
			// The native compile capture places the intact row on line 6.
			return "P pq=new P();\n\n\n\n\n" + tc.Code
		}
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		return prefix + data.Prelude + "\ntry {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
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
			return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, item.Message)
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SOQL selection", api)
			root := t.TempDir()
			names := make([]string, 0, len(data.Declarations)+1)
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
			}
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
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("helper semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			// Each method gets isolated runner state; compile the named matrix once.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class SOQLSelectionProbe {\n")
			executable := 0
			for _, tc := range data.Cases {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "SOQLSelectionProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "SOQLSelectionProbe" || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if run.Summary().Total != executable || len(namedResults) != executable {
				t.Fatalf("named results: %#v, unique %d expected %d: %s", run.Summary(), len(namedResults), executable, firstRunProblem(run))
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					counts.run(t, "anonymous", "anonymous", "exact", func(t *testing.T) {
						source := body(tc)
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
						_, err = runner.execute(program)
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
					})
					counts.run(t, "isTest", "@IsTest", "exact", func(t *testing.T) {
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							className := "SOQLSelectionCompile" + tc.ID
							source := "@IsTest private class " + className + " {\n@IsTest static void observed(){\n\n\nP pq=new P();\n" + tc.Code + "\n}\n}\n"
							path := filepath.Join(root, className+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							analysis := sema.Analyze(buildIndex(path))
							if observed := compileText(analysis); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
						result := namedResults[tc.ID]
						if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
							observed := "UNCAUGHT|"
							if result.Problem != nil {
								observed += result.Problem.Type + ": " + result.Problem.Message
							}
							if result.Status != testreport.StatusFail || observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s> status %s", tc.Expected, observed, result.Status)
							}
							return
						}
						if result.Status != testreport.StatusPass {
							t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
						}
					})
				})
			}
		})
	}
}
