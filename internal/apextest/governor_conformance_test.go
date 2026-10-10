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

// The exported owned oracle needs no Salesforce connection or probe project.
// Every asserted result is exact text or raw null, on both execution routes.
func TestGovernorOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
		NativeLine          int
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Cases        []familyCase
		Controls     []familyCase
		Remaining    []struct {
			ID, Owner, Reason string
			APIVersions       []string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/governor.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 291 || len(data.Controls) != 4 || len(data.Remaining) != 0 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle: %d rows + %d controls + %d carries at %v", len(data.Cases), len(data.Controls), len(data.Remaining), data.APIVersions)
	}
	allRows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, tc := range allRows {
		if tc.ID == "" || seen[tc.ID] || tc.Expected == "?" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && tc.NativeLine == 0 {
			t.Fatalf("invalid asserted row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	for _, row := range data.Remaining {
		if row.ID == "" || row.Owner == "" || row.Owner == "A29" || row.Reason == "" || len(row.APIVersions) == 0 {
			t.Fatalf("invalid carried row: %#v", row)
		}
		if seen[row.ID] && strings.Join(row.APIVersions, ",") != "62.0" {
			t.Fatalf("asserted row carried at every API: %#v", row)
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
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	rejectedSource := func(tc familyCase, named bool) string {
		prefix := ""
		if named {
			prefix = "@IsTest private class GovernorCompile" + tc.ID + " {\n@IsTest static void observed(){\n"
		}
		prefix += "P pq=new P();\n"
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
	splitDeclarations := func(code string) (map[string]string, string) {
		parsed := apexast.NewParser().ParseSource("GovernorAnonymous.cls", code)
		declarations := map[string]string{}
		masked := []byte(code)
		for _, decl := range parsed.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			default:
				continue
			}
			start, end := decl.Range.Start.Offset, decl.Range.End.Offset
			if start < 0 || end <= start || end > len(code) {
				t.Fatalf("invalid declaration range: %s", decl.Name)
			}
			declarations[decl.Name] = code[start:end]
			for i := start; i < end; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
		return declarations, string(masked)
	}
	compileText := func(result sema.Result) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			message := item.Message
			if item.NativeMessage != "" {
				message = item.NativeMessage
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			if item.NativeLine != nil {
				line = *item.NativeLine
			}
			return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "governor limits", api)
			carried := map[string]bool{}
			for _, row := range data.Remaining {
				for _, version := range row.APIVersions {
					if version == api {
						if carried[row.ID] {
							t.Fatalf("repeated carry: %s", row.ID)
						}
						carried[row.ID] = true
						t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
					}
				}
			}
			var rows []familyCase
			for _, tc := range allRows {
				if !carried[tc.ID] {
					rows = append(rows, tc)
				}
			}
			root := t.TempDir()
			names := []string{"P"}
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
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
			buildIndex := func(extra ...string) typesys.Index {
				files := append([]string{}, paths...)
				files = append(files, extra...)
				return typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, gladeschema.Schema{})
			}
			index := buildIndex()
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
			namedSource.WriteString("@IsTest private class GovernorProbe {\n")
			executable := 0
			for _, tc := range rows {
				if tc.NativeLine > 0 {
					continue
				}
				executable++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "GovernorProbe.cls")
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
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{LinkProject: true, Org: &org, Base: anonymousRunner})
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			runtimeMethods := indexTestRuntimeMethods(methods)
			counters := newRunPerfCounters(false)
			namedByID := make(map[string]TestCase, len(namedCases))
			for _, tc := range namedCases {
				id := strings.TrimPrefix(tc.MethodName, "observed")
				if tc.ClassName != "GovernorProbe" || namedByID[id].MethodName != "" {
					t.Fatalf("unexpected/repeated named row: %#v", tc)
				}
				key := testCaseKey(tc)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named compile %s: %v %v", id, methodErrors[key], programErrors[key])
				}
				namedByID[id] = tc
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					decls, code := map[string]string{}, tc.Code
					if tc.NativeLine > 0 {
						decls, code = splitDeclarations(tc.Code)
					}
					declNames := make([]string, 0, len(decls))
					for name := range decls {
						declNames = append(declNames, name)
					}
					sort.Strings(declNames)
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						if len(decls) > 0 {
							// Like exec and A04, lift declarations before analyzing
							// the anonymous body, retaining the captured row line.
							var declarationPaths []string
							for _, name := range declNames {
								path := filepath.Join(root, name+".cls")
								writeFile(t, path, strings.Repeat("\n", tc.NativeLine-1)+decls[name])
								writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
								declarationPaths = append(declarationPaths, path)
							}
							analysis := sema.AnalyzeAnonymousDeclarations(sema.WithAnonymousDeclarationContext(buildIndex(declarationPaths...)))
							if observed := compileText(analysis); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
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
						if _, err := anonymousRunner.execute(program, func(machine *vm.VM) { machine.SetTraceEnabled(true) }); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						counts.Track(t, "@IsTest", 1)
						if tc.NativeLine > 0 {
							path := filepath.Join(root, "GovernorCompile"+tc.ID+".cls")
							source := rejectedSource(tc, true)
							if len(decls) > 0 {
								// A class declaration belongs in the test class scope,
								// while the captured statements remain in its method.
								source = "@IsTest private class GovernorCompile" + tc.ID + " {\n"
								source += strings.Repeat("\n", tc.NativeLine-2)
								for _, name := range declNames {
									source += decls[name] + "\n"
								}
								source += "@IsTest static void observed(){\nP pq=new P();\n" + code + "\n}\n}\n"
							}
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							if observed := compileText(sema.Analyze(buildIndex(path))); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
						row := namedByID[tc.ID]
						key := testCaseKey(row)
						seed := namedRunner.newOrg()
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
