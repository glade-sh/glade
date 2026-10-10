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

// A39's captured API 62/67 rows, owned metadata and native broker observations.
// CI reconstructs the fixtures from this export and never imports glade-tools.
func TestPlatformEventsOrgConformance(t *testing.T) {
	type namedCompilation struct{ Name, Source, Expected string }
	type familyCase struct {
		ID, Code, Expected, Owner, Reason string
		Compile, NativeNull               bool
		NativeLine                        int
		NamedCompilations                 map[string]namedCompilation
	}
	type nativeTest struct{ Class, Method, Outcome, Message, Owner, Reason string }
	var data struct {
		APIVersions                 []string `json:"apiVersions"`
		Files, NamedObservations    map[string]string
		Cases, Controls             []familyCase
		Remaining, Excluded         []familyCase
		NativeTests, NativeExcluded []nativeTest
	}
	raw, err := os.ReadFile("testdata/conformance/platform_events.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 243 || len(data.Controls) != 83 || len(data.NativeTests) != 56 || len(data.Remaining) != 0 || len(data.Excluded) != 12 || len(data.NativeExcluded) != 2 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle: %d asserted + %d controls, %d native tests, %d carries, %d oracle exclusions, %d hosted test exclusions at %v", len(data.Cases), len(data.Controls), len(data.NativeTests), len(data.Remaining), len(data.Excluded), len(data.NativeExcluded), data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ID == "" || seen[row.ID] || row.Expected == "?" || row.Owner != "" || strings.HasPrefix(row.Expected, "COMPILE_ERROR") && row.NativeLine == 0 {
			t.Fatalf("invalid asserted row: %#v", row)
		}
		seen[row.ID] = true
		if len(row.NamedCompilations) != 0 {
			for _, api := range data.APIVersions {
				capture, ok := row.NamedCompilations[api]
				if !ok || capture.Name == "" || capture.Source == "" || !strings.HasPrefix(capture.Expected, "COMPILE_ERROR\t") {
					t.Fatalf("invalid named rejection capture for %s/%s", row.ID, api)
				}
			}
		}
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A39" || row.Reason == "" {
			t.Fatalf("invalid carried row: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
	}
	for _, row := range data.Excluded {
		if row.ID == "" || seen[row.ID] || row.Reason == "" {
			t.Fatalf("invalid oracle exclusion: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("excluded %s: %s", row.ID, row.Reason)
	}
	for _, row := range data.NativeExcluded {
		t.Logf("excluded native %s.%s: %s", row.Class, row.Method, row.Reason)
	}
	// Preserve the native TSV newline encoding and compare case and Id length.
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
  if(expectedNull){System.assert(v==null,id+' expected raw null actual <'+observedText+'>');return;}
  if(expectedText=='null'){System.assert(v!=null,id+' expected String null actual raw null');}
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	declarations := func(code string) (map[string]string, string) {
		parsed := apexast.NewParser().ParseSource("PlatformEventsAnonymous.cls", code)
		out := map[string]string{}
		masked := []byte(code)
		for _, decl := range parsed.Declarations {
			switch decl.Kind {
			case apexast.DeclarationClass, apexast.DeclarationInterface, apexast.DeclarationEnum:
			default:
				continue
			}
			start, end := decl.Range.Start.Offset, decl.Range.End.Offset
			if start < 0 || end <= start || end > len(code) {
				t.Fatalf("invalid declaration range in %q", code)
			}
			out[decl.Name] = code[start:end]
			for i := start; i < end; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
		return out, string(masked)
	}
	body := func(tc familyCase, expected string) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			_, code := declarations(tc.Code)
			return prefix + code
		}
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	rejectedSource := func(tc familyCase, named bool) string {
		prefix := ""
		if named {
			prefix = "@IsTest private class PlatformEventsCompile" + tc.ID + " {\n@IsTest static void observed(){\n"
		}
		prefix += "P pq=new P();\n"
		line := strings.Count(prefix, "\n") + 1
		if tc.NativeLine < line {
			t.Fatalf("native line %d precedes rejection body for %s", tc.NativeLine, tc.ID)
		}
		prefix += strings.Repeat("\n", tc.NativeLine-line)
		if !tc.Compile {
			prefix += "Object r; "
		}
		prefix += tc.Code
		if named {
			prefix += "\n}\n}\n"
		}
		return prefix
	}
	compileText := func(result sema.Result, withLine bool) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			if item.NativeLine != nil {
				line = *item.NativeLine
			}
			message := item.Message
			if item.NativeMessage != "" {
				message = item.NativeMessage
			}
			observed := strings.ReplaceAll(message, "\n", " ")
			if withLine {
				observed = fmt.Sprintf("line %d: %s", line, observed)
			}
			if len(observed) > 300 {
				observed = observed[:300]
			}
			return "COMPILE_ERROR\t" + observed
		}
		return "ACCEPTED"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "platform events", api, "")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			names := make([]string, 0, len(data.Files))
			for name := range data.Files {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				source := data.Files[name]
				if strings.HasSuffix(name, "-meta.xml") {
					source = strings.ReplaceAll(source, "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>")
				}
				writeFile(t, filepath.Join(root, name), source)
			}
			helperPath := filepath.Join(root, "force-app/main/default/classes/P.cls")
			writeFile(t, helperPath, helper)
			writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			fixtureProject, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			fixtureSchema, err := gladeschema.LoadProject(fixtureProject)
			if err != nil {
				t.Fatal(err)
			}
			buildIndex := func(extra ...string) typesys.Index {
				p := fixtureProject
				p.ApexFiles = append(append([]string{}, p.ApexFiles...), extra...)
				return typesys.Build(p, fixtureSchema)
			}
			analyzeNamedRejection := func(tc familyCase, declPaths []string) sema.Result {
				if capture, ok := tc.NamedCompilations[api]; ok {
					// C029/C030: keep the native named wrapper and nested declaration
					// byte-identical, rather than turning CB into a top-level type.
					path := filepath.Join(root, capture.Name+".cls")
					writeFile(t, path, capture.Source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					return sema.Analyze(buildIndex(path))
				}
				// Named Apex keeps rejected top-level declarations in their own
				// source files; the test body contains the unchanged remaining text.
				_, tc.Code = declarations(tc.Code)
				path := filepath.Join(root, "PlatformEventsCompile"+tc.ID+".cls")
				writeFile(t, path, rejectedSource(tc, true))
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths := append(append([]string{}, declPaths...), path)
				return sema.Analyze(buildIndex(paths...))
			}
			assertRejection := func(t *testing.T, tc familyCase, analysis sema.Result, named bool) {
				t.Helper()
				expected, withLine := tc.Expected, true
				if named {
					if capture, ok := tc.NamedCompilations[api]; ok {
						// Tooling ApexClass insertion returns the bare native diagnostic.
						expected, withLine = capture.Expected, false
					}
				}
				if observed := compileText(analysis, withLine); observed != expected {
					t.Fatalf("expected <%s> actual <%s>", expected, observed)
				}
			}
			index := buildIndex()
			if index.HasErrors() {
				t.Fatalf("fixture parser: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("fixture semantics: %v", analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			requestRuntime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
			if err != nil {
				t.Fatal(err)
			}
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &requestRuntime, Org: &org})
			// Inline declarations extend private clones of the shared API base.
			declarationResults := map[string]sema.Result{}
			declarationPaths := map[string][]string{}
			declarationIndexes := map[string]typesys.Index{}
			declarationRunners := map[string]*conformanceRunner{}
			namedPaths := []string{}
			namedClassByID := map[string]string{}
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class PlatformEventsProbe {\n")
			executable := 0
			for _, tc := range rows {
				decls, _ := declarations(tc.Code)
				if len(decls) > 0 {
					declRoot := t.TempDir()
					declPaths := []string{}
					for name, source := range decls {
						if tc.NativeLine > 0 {
							source = strings.Repeat("\n", tc.NativeLine-1) + source
						}
						path := filepath.Join(declRoot, name+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						declPaths = append(declPaths, path)
					}
					transient := typesys.Build(project.Project{Root: declRoot, ApexFiles: declPaths, SourceAPIVersion: api}, fixtureSchema)
					if transient.HasErrors() {
						t.Fatalf("%s declaration parser: %v", tc.ID, transient.Diagnostics)
					}
					// Retain authoritative source digests for the new declarations.
					merged := buildIndex(declPaths...)
					declarationResults[tc.ID] = sema.AnalyzeAnonymousDeclarationsInContext(merged, transient)
					declarationPaths[tc.ID] = declPaths
					declarationIndexes[tc.ID] = merged
					if tc.NativeLine == 0 {
						declarationRunners[tc.ID] = newConformanceRunner(t, merged, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
					}
				}
				if tc.NativeLine > 0 {
					continue
				}
				executable++
				expected := tc.Expected
				if observed, ok := data.NamedObservations[tc.ID]; ok {
					expected = observed
				}
				className := "PlatformEventsProbe"
				if len(decls) == 0 {
					fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, expected))
				} else {
					className = "PlatformEvents" + tc.ID
					var source strings.Builder
					fmt.Fprintf(&source, "@IsTest private class %s {\n", className)
					declNames := make([]string, 0, len(decls))
					for name := range decls {
						declNames = append(declNames, name)
					}
					sort.Strings(declNames)
					for _, name := range declNames {
						source.WriteString(decls[name] + "\n")
					}
					fmt.Fprintf(&source, "@IsTest static void observed%s(){\n%s\n}\n}\n", tc.ID, body(tc, expected))
					path := filepath.Join(root, className+".cls")
					writeFile(t, path, source.String())
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					namedPaths = append(namedPaths, path)
				}
				namedClassByID[tc.ID] = className
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "PlatformEventsProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedPaths = append(namedPaths, namedPath)
			namedIndex := buildIndex(namedPaths...)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			selected := map[string]bool{"PlatformEventsProbe": true}
			for _, class := range namedClassByID {
				selected[class] = true
			}
			for _, row := range data.NativeTests {
				selected[row.Class] = true
			}
			selectedClasses := []string{}
			for class := range selected {
				selectedClasses = append(selectedClasses, class)
			}
			sort.Strings(selectedClasses)
			namedCases := Discover(namedIndex, Options{SelectedClasses: selectedClasses})
			// The two hosted internal-error outcomes remain excluded at invocation.
			if len(namedCases) != executable+len(data.NativeTests)+len(data.NativeExcluded) {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), executable+len(data.NativeTests)+len(data.NativeExcluded))
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			runtimeMethods := indexTestRuntimeMethods(methods)
			counters := newRunPerfCounters(false)
			byName := map[string]TestCase{}
			for _, row := range namedCases {
				key := row.ClassName + "." + row.MethodName
				if _, exists := byName[key]; exists {
					t.Fatalf("duplicate named method: %s", key)
				}
				byName[key] = row
			}
			runNamed := func(class, method string) testreport.Case {
				row, ok := byName[class+"."+method]
				if !ok {
					t.Fatalf("missing named method: %s.%s", class, method)
				}
				key := testCaseKey(row)
				seed := namedRunner.org.CloneRuntimeOrg()
				initializeTestOrg(&seed)
				return runCase(context.Background(), row, methods[key], runtimeMethods[testMethodSourceKey(row.ClassName, row.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					for _, named := range []bool{false, true} {
						route := "anonymous"
						if named {
							route = "isTest"
						}
						counts.Run(t, route, tc.ID, route, func(t *testing.T) {
							if analysis, ok := declarationResults[tc.ID]; ok {
								if tc.NativeLine > 0 {
									if named {
										analysis = analyzeNamedRejection(tc, declarationPaths[tc.ID])
									}
									assertRejection(t, tc, analysis, named)
									return
								}
								if analysis.HasErrors() {
									t.Fatalf("accepted declaration: %v", analysis.Diagnostics)
								}
							}
							if tc.NativeLine > 0 {
								analysis := sema.Result{}
								if named {
									analysis = analyzeNamedRejection(tc, nil)
								} else {
									analysis = sema.AnalyzeAnonymous(index, rejectedSource(tc, false), api)
								}
								assertRejection(t, tc, analysis, named)
								return
							}
							if named {
								if result := runNamed(namedClassByID[tc.ID], "observed"+tc.ID); result.Status != testreport.StatusPass {
									t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
								}
								return
							}
							source := body(tc, tc.Expected)
							rowIndex, runner := index, anonymousRunner
							if declaration, ok := declarationIndexes[tc.ID]; ok {
								rowIndex, runner = declaration, declarationRunners[tc.ID]
							}
							if analysis := sema.AnalyzeAnonymous(rowIndex, source, api); analysis.HasErrors() {
								t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
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
			for _, tc := range data.NativeTests {
				kind := "exact"
				if tc.Outcome == "Pass" {
					kind = "category"
				}
				counts.RunKind(t, "isTest", "native-"+tc.Class+"-"+tc.Method, "native-"+tc.Class+"-"+tc.Method, kind, func(t *testing.T) {
					result := runNamed(tc.Class, tc.Method)
					if tc.Outcome == "Pass" {
						if result.Status != testreport.StatusPass {
							t.Fatalf("native passed; local status %s problem %v", result.Status, result.Problem)
						}
						return
					}
					observed := ""
					if result.Problem != nil {
						typeName := result.Problem.Type
						if !strings.Contains(typeName, ".") {
							typeName = "System." + typeName
						}
						observed = typeName + ": " + result.Problem.Message
					}
					if tc.Outcome != "Fail" || result.Status != testreport.StatusFail || observed != tc.Message {
						t.Fatalf("native %s <%s>; local status %s actual <%s> problem %v", tc.Outcome, tc.Message, result.Status, observed, result.Problem)
					}
				})
			}
		})
	}
}
