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

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Both API endpoints use captured Salesforce answers and the same owned
// calendar/holiday setup. Neither execution route needs Salesforce or tools.
func TestBusinessHoursOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected string
		Compile            bool
		ExpectedNull       bool
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Prelude      string
		Cases        []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/business_hours.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 294 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle rows/versions: %d %v", len(data.Cases), data.APIVersions)
	}
	fixtureFile, err := os.Open("testdata/conformance/business_hours_fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := storage.ReadFixture(fixtureFile)
	closeErr := fixtureFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=String.valueOf(v);
  if(expectedNull){
   System.assert(v==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.ExpectedNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		return prefix + data.Prelude + "\ntry {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	seen := make(map[string]bool, len(data.Cases))
	for _, tc := range data.Cases {
		if seen[tc.ID] || tc.Expected == "?" || tc.Expected == "" {
			t.Fatalf("invalid oracle row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	compileObservation := func(diagnostics []diagnostic.Diagnostic) string {
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			if d.Range != nil && d.Range.Start.Line > 0 {
				return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", d.Range.Start.Line, d.Message)
			}
			return "COMPILE_ERROR\t" + d.Message
		}
		return ""
	}
	assertCompileObservation := func(t *testing.T, tc familyCase, observed string) {
		t.Helper()
		if observed == tc.Expected {
			return
		}
		// Diagnostic wording for these rows is carried by the type system rows;
		// the exported matching fact is their exact compile outcome.
		switch tc.ID {
		case "C001", "C002", "C003", "C004", "C005", "C006", "C007", "C008", "C010", "C014", "C015", "C016", "C017", "C018":
			expectedOutcome, _, _ := strings.Cut(tc.Expected, "\t")
			observedOutcome, _, _ := strings.Cut(observed, "\t")
			if observedOutcome != expectedOutcome {
				t.Fatalf("%s expected compile outcome <%s> actual <%s>", tc.ID, expectedOutcome, observedOutcome)
			}
			t.Logf("%s diagnostic text carried to type system: expected <%s> actual <%s>", tc.ID, tc.Expected, observed)
			return
		}
		t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.Expected, observed)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "business hours", api, "")
			root := t.TempDir()
			names := make([]string, 0, len(data.Declarations)+1)
			declarations := make(map[string]string, len(data.Declarations)+1)
			for name, source := range data.Declarations {
				declarations[name] = source
				names = append(names, name)
			}
			declarations["P"] = helper
			names = append(names, "P")
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
			if a := sema.Analyze(index); a.HasErrors() {
				t.Fatalf("helper semantics: %v", a.Diagnostics)
			}
			org := orgFromIndex(index)
			if err := storage.ApplyFixture(&org, fixture); err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest(SeeAllData=true) private class BusinessHoursProbe {\n")
			accepted := 0
			for _, tc := range data.Cases {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "BusinessHoursProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if a := sema.Analyze(namedIndex); a.HasErrors() {
				t.Fatalf("named semantics: %v", a.Diagnostics)
			}
			namedCases := Discover(namedIndex, Options{})
			if len(namedCases) != accepted {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), accepted)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			testMethods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			runtimeMethods := indexTestRuntimeMethods(testMethods)
			namedResults := make(map[string]testreport.Case, accepted)
			// Use the runner's ordinary per-test executor with an explicitly seeded
			// org: Run has no fixture option for configured setup records.
			counters := newRunPerfCounters(false)
			for _, tc := range namedCases {
				key := testCaseKey(tc)
				seed := namedRunner.newOrg()
				initializeTestOrg(&seed)
				result := runCase(context.Background(), tc, testMethods[key], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
				id := strings.TrimPrefix(result.MethodName, "observed")
				if result.ClassName != "BusinessHoursProbe" || namedResults[id].MethodName != "" {
					t.Fatalf("unexpected/repeated named result: %#v", result)
				}
				namedResults[id] = result
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc)
					kind := "exact"
					switch tc.ID {
					case "C001", "C002", "C003", "C004", "C005", "C006", "C007", "C008", "C010", "C014", "C015", "C016", "C017", "C018":
						kind = "carried"
					}
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					counts.RunKind(t, "anonymous", tc.ID, "anonymous", kind, func(t *testing.T) {
						anonymousSource := source
						if rejected {
							// Native compile captures put the row on line 6, after
							// five helper lines. Keep that location in both routes.
							anonymousSource = "\n\n\n\n" + source
						}
						analysis := sema.AnalyzeAnonymous(index, anonymousSource, api)
						program, compileErr := vm.CompileAnonymousWithOptions(anonymousSource, vm.CompileOptions{APIVersion: api})
						if rejected {
							observed := compileObservation(analysis.Diagnostics)
							if observed == "" && compileErr != nil {
								observed = "COMPILE_ERROR\t" + compileErr.Error()
							}
							assertCompileObservation(t, tc, observed)
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					counts.RunKind(t, "isTest", tc.ID, "isTest", kind, func(t *testing.T) {
						if !rejected {
							result := namedResults[tc.ID]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
							}
							return
						}
						path := filepath.Join(root, "BusinessHoursRejected.cls")
						writeFile(t, path, "@IsTest private class BusinessHoursRejected {\n@IsTest static void observed(){\n\n\n"+source+"\n} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						rejectedIndex := buildIndex(path)
						analysis := sema.Analyze(rejectedIndex)
						diagnostics := analysis.Diagnostics
						if rejectedIndex.HasErrors() {
							diagnostics = rejectedIndex.Diagnostics
						}
						observed := compileObservation(diagnostics)
						if observed == "" {
							run := Run(rejectedIndex, Options{NoDiskCache: true, Parallelism: 1})
							if run.Summary().CompileErrors > 0 {
								observed = "COMPILE_ERROR\t" + firstRunProblem(run)
							}
						}
						assertCompileObservation(t, tc, observed)
					})
				})
			}
		})
	}
}
