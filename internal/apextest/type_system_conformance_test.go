package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Captured type-system observations need neither Salesforce nor glade-tools in CI.
func TestTypeSystemOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Group, Code, Expected string
		Owner, Reason             string
	}
	var data struct {
		APIVersions           []string `json:"apiVersions"`
		Declarations          map[string]string
		AnonymousDeclarations map[string]string `json:"anonymousDeclarations"`
		Cases, Remaining      []familyCase
		DiagnosticTextOwner   string              `json:"diagnosticTextOwner"`
		CaseRows              map[string][]string `json:"caseRows"`
	}
	raw, err := os.ReadFile("testdata/conformance/type_system.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 277 || len(data.Remaining) != 3 || len(data.CaseRows["all"]) != 280 || strings.Join(data.APIVersions, ",") != "62.0,67.0" || data.DiagnosticTextOwner != "A01" {
		t.Fatalf("oracle shape: cases=%d carried=%d listed=%d versions=%v diagnostic owner=%s", len(data.Cases), len(data.Remaining), len(data.CaseRows["all"]), data.APIVersions, data.DiagnosticTextOwner)
	}
	seen := map[string]bool{}
	for _, row := range data.Cases {
		if seen[row.ID] || row.ID == "" || row.Code == "" {
			t.Fatalf("duplicate or incomplete assertion row: %#v", row)
		}
		seen[row.ID] = true
	}
	for _, row := range data.Remaining {
		if seen[row.ID] || row.ID == "" || row.Owner == "" || row.Reason == "" {
			t.Fatalf("invalid carried row: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("%s carried to %s: %s; native <%s>", row.ID, row.Owner, row.Reason, row.Expected)
	}
	for _, id := range data.CaseRows["all"] {
		if !seen[id] {
			t.Fatalf("listed case row has no disposition: %s", id)
		}
	}
	// Compare exact text; Apex == and assertEquals erase case and Id differences.
	// A native null must also be the raw value, not the string "null".
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
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
		if tc.Expected == "null" {
			prefix += "pq.expectedNull=true;\n"
		}
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	assertRejection := func(t *testing.T, tc familyCase, rejected bool) {
		t.Helper()
		expectedText, diagnosticText, _ := strings.Cut(tc.Expected, "\t")
		observedText := "compiled"
		if rejected {
			observedText = "COMPILE_ERROR"
		}
		if expectedText != observedText {
			t.Fatalf("%s expected <%s> actual <%s>", tc.ID, expectedText, observedText)
		}
		// Source offsets and diagnostic wording are carried, never asserted.
		t.Logf("%s diagnostic text carried to %s: native <%s>; exact rejection category matched", tc.ID, data.DiagnosticTextOwner, diagnosticText)
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "type system", api, "")
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
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if a := sema.Analyze(index); a.HasErrors() {
				t.Fatalf("helper semantics: %v", a.Diagnostics)
			}
			// Keep captured anonymous declarations intact. Public companion
			// classes need explicit virtual modifiers on their base types, which
			// would otherwise hide R137's anonymous interface/base checking path.
			anonymousRoot := t.TempDir()
			anonymousPaths := make([]string, 0, len(names))
			for _, name := range names {
				source := data.AnonymousDeclarations[name]
				if name == "P" {
					source = helper
				}
				if source == "" {
					t.Fatalf("missing captured anonymous declaration: %s", name)
				}
				path := filepath.Join(anonymousRoot, name+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				anonymousPaths = append(anonymousPaths, path)
			}
			anonymousIndex := typesys.Build(project.Project{Root: anonymousRoot, SourceAPIVersion: api, ApexFiles: anonymousPaths}, gladeschema.Schema{})
			if anonymousIndex.HasErrors() {
				t.Fatalf("anonymous helper parser: %v", anonymousIndex.Diagnostics)
			}
			if a := sema.AnalyzeAnonymousDeclarations(anonymousIndex); a.HasErrors() {
				t.Fatalf("anonymous helper semantics: %v", a.Diagnostics)
			}
			runner := newConformanceRunner(t, anonymousIndex, conformanceRunnerOptions{LinkProject: true})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class TypeSystemProbe {\n")
			accepted := 0
			for _, tc := range data.Cases {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			var namedResults map[string]testreport.Case
			prepareNamedResults := func(t *testing.T) {
				t.Helper()
				if namedResults != nil {
					return
				}
				// Rejected-row retries avoid running the accepted-row matrix.
				path := filepath.Join(root, "TypeSystemProbe.cls")
				writeFile(t, path, namedSource.String())
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				namedIndex := buildIndex(path)
				if namedIndex.HasErrors() {
					t.Fatalf("named parser: %v", namedIndex.Diagnostics)
				}
				if a := sema.Analyze(namedIndex); a.HasErrors() {
					t.Fatalf("named semantics: %v", a.Diagnostics)
				}
				run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
				namedResults = map[string]testreport.Case{}
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						id := strings.TrimPrefix(result.MethodName, "observed")
						if result.ClassName != "TypeSystemProbe" || namedResults[id].MethodName != "" {
							t.Fatalf("unexpected/repeated named result: %#v", result)
						}
						namedResults[id] = result
					}
				}
				if run.Summary().Total != accepted || len(namedResults) != accepted {
					t.Fatalf("named results: %#v unique=%d expected=%d: %s", run.Summary(), len(namedResults), accepted, firstRunProblem(run))
				}
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if rejected {
						kind = "category"
					}
					counts.RunKind(t, "anonymous", tc.ID, "anonymous", kind, func(t *testing.T) {
						analysis := sema.AnalyzeAnonymous(anonymousIndex, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if rejected {
							assertRejection(t, tc, compileErr != nil || analysis.HasErrors())
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
							prepareNamedResults(t)
							result := namedResults[tc.ID]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status=%s problem=%v", result.Status, result.Problem)
							}
							return
						}
						path := filepath.Join(root, "TypeSystemRejected.cls")
						writeFile(t, path, "@IsTest private class TypeSystemRejected { @IsTest static void observed(){"+source+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						rejectedIndex := buildIndex(path)
						analysis := sema.Analyze(rejectedIndex)
						assertRejection(t, tc, rejectedIndex.HasErrors() || analysis.HasErrors())
					})
				})
			}
		})
	}
}
