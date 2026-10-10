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

// Owned API 62/67 observations. Callback value rows and native test-drain
// outcomes are separate assertions; only the latter establish async execution.
func TestAsyncOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
		NativeLine          int
		MatchFields         []string
	}
	var data struct {
		APIVersions                  []string `json:"apiVersions"`
		Declarations                 map[string]string
		Cases, Controls, NativeCases []familyCase
		CronControls                 []familyCase
		ShadowDeclarations           map[string]string
		ShadowCases                  []familyCase
		NamedObservations            map[string]string
		NativeTests                  []struct{ Method, Outcome, Message string }
		Remaining                    []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/async_execution.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 228 || len(data.Controls) != 44 || len(data.NativeCases) != 47 || len(data.CronControls) != 3 || len(data.ShadowCases) != 4 || len(data.ShadowDeclarations) != 1 || len(data.NativeTests) != 10 || len(data.Remaining) != 0 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle: %d original rows, %d adjacent controls, %d native controls, %d Cron controls, %d shadow controls, %d shadow declarations, %d drain outcomes, %d carries at %v", len(data.Cases), len(data.Controls), len(data.NativeCases), len(data.CronControls), len(data.ShadowCases), len(data.ShadowDeclarations), len(data.NativeTests), len(data.Remaining), data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	nativeRows := append(append([]familyCase{}, data.NativeCases...), data.CronControls...)
	seen := map[string]bool{}
	for _, tc := range append(append(append([]familyCase{}, rows...), nativeRows...), data.ShadowCases...) {
		if tc.ID == "" || seen[tc.ID] || tc.Expected == "?" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && tc.NativeLine == 0 {
			t.Fatalf("invalid asserted row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	// T046-T048 retain the complete native observation and unchanged row body.
	// Assert the captured Cron fields; callback/count checks remain in the
	// existing local drain fixtures.
	for _, tc := range data.CronControls {
		if len(tc.MatchFields) == 0 {
			t.Fatalf("missing Cron observation fields: %s", tc.ID)
		}
		t.Logf("%s Cron fields asserted: %v; full native observation: %s", tc.ID, tc.MatchFields, tc.Expected)
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A38" || row.Reason == "" {
			t.Fatalf("invalid carried row: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
	}
	// Native TSV cells use the probe transport's literal newline escapes.
	// Preserve the cells and apply that same encoding before exact comparisons.
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public List<String> matchFields;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
  if(expectedNull){
   System.assert(v==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  if(matchFields!=null){
   List<String> matchedValues=new List<String>();
   for(String part:observedText.split('\\|')){
    Integer equalsAt=part.indexOf('=');
    if(equalsAt>=0 && matchFields.contains(part.substring(0,equalsAt))) matchedValues.add(part);
   }
   observedText=String.join(matchedValues,'|');
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	// As in source syntax conformance, transient declarations pass the real
	// anonymous declaration gate before either route. Their unchanged bodies
	// live in companion files; masking leaves executable source offsets intact.
	declarations := func(code string) (map[string]string, string) {
		parsed := apexast.NewParser().ParseSource("AsyncAnonymous.cls", code)
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
		fields := ""
		if len(tc.MatchFields) > 0 {
			wanted := map[string]bool{}
			for _, key := range tc.MatchFields {
				if wanted[key] {
					t.Fatalf("duplicate observation field %s: %s", tc.ID, key)
				}
				wanted[key] = true
				if fields != "" {
					fields += ","
				}
				fields += conformanceApexString(key)
			}
			matched := []string{}
			for _, part := range strings.Split(expected, "|") {
				key, _, ok := strings.Cut(part, "=")
				if ok && wanted[key] {
					matched = append(matched, part)
				}
			}
			if len(matched) != len(tc.MatchFields) {
				t.Fatalf("missing captured observation fields %s: %s", tc.ID, expected)
			}
			expected = strings.Join(matched, "|")
		}
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(expected) + ";\n"
		if fields != "" {
			prefix += "pq.matchFields=new List<String>{" + fields + "};\n"
		}
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		code := tc.Code
		if tc.Compile {
			_, code = declarations(code)
			return prefix + code
		}
		return prefix + "try {Object r; " + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	rejectedSource := func(tc familyCase, named bool) string {
		prefix := ""
		if named {
			prefix = "@IsTest private class AsyncCompile" + tc.ID + " {\n@IsTest static void observed(){\n"
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
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "async Apex", api)
			root := t.TempDir()
			names := []string{"P"}
			for name := range data.Declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			paths := make([]string, 0, len(names))
			for _, name := range names {
				source := data.Declarations[name]
				if name == "P" {
					source = helper
				}
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, source)
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
				t.Fatalf("fixture parser: %v", index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("fixture semantics: %v", analysis.Diagnostics)
			}
			declarationResults := map[string]sema.Result{}
			for _, tc := range rows {
				if !tc.Compile {
					continue
				}
				decls, _ := declarations(tc.Code)
				if len(decls) == 0 {
					continue
				}
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
				transient := typesys.Build(project.Project{Root: declRoot, ApexFiles: declPaths, SourceAPIVersion: api}, gladeschema.Schema{})
				if transient.HasErrors() {
					t.Fatalf("%s declaration parser: %v", tc.ID, transient.Diagnostics)
				}
				merged := index
				merged.Types = append(append([]typesys.TypeSymbol(nil), index.Types...), transient.Types...)
				declarationResults[tc.ID] = sema.AnalyzeAnonymousDeclarationsInContext(merged, transient)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var namedRunner *conformanceRunner
			var runNamed func(string, string) testreport.Case
			prepareNamed := func(t *testing.T) {
				t.Helper()
				if namedRunner != nil {
					return
				}
				var source strings.Builder
				source.WriteString("@IsTest private class AsyncProbe {\n")
				executable := 0
				for _, tc := range append(append([]familyCase{}, rows...), nativeRows...) {
					if tc.NativeLine > 0 {
						continue
					}
					executable++
					expected := tc.Expected
					if observed, ok := data.NamedObservations[tc.ID]; ok {
						expected = observed
					}
					fmt.Fprintf(&source, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, expected))
				}
				source.WriteString("}\n")
				path := filepath.Join(root, "AsyncProbe.cls")
				writeFile(t, path, source.String())
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				namedIndex := buildIndex(path)
				if namedIndex.HasErrors() {
					t.Fatalf("named parser: %v", namedIndex.Diagnostics)
				}
				if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
					t.Fatalf("named semantics: %v", analysis.Diagnostics)
				}
				namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"AsyncProbe", "AsyncA38DrainTest"}})
				if len(namedCases) != executable+len(data.NativeTests) {
					t.Fatalf("named discovery: %d expected %d", len(namedCases), executable+len(data.NativeTests))
				}
				namedRunner = newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
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
				runNamed = func(class, method string) testreport.Case {
					row, ok := byName[class+"."+method]
					if !ok {
						t.Fatalf("missing named method: %s.%s", class, method)
					}
					key := testCaseKey(row)
					seed := namedRunner.org.CloneRuntimeOrg()
					initializeTestOrg(&seed)
					return runCase(context.Background(), row, methods[key], runtimeMethods[testMethodSourceKey(row.ClassName, row.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
				}
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					for _, named := range []bool{false, true} {
						route := "anonymous"
						if named {
							route = "isTest"
						}
						t.Run(route, func(t *testing.T) {
							countRoute := "anonymous"
							if named {
								countRoute = "@IsTest"
							}
							counts.Track(t, countRoute, 1)
							if declaration, ok := declarationResults[tc.ID]; ok {
								if tc.NativeLine > 0 {
									if observed := compileText(declaration); observed != tc.Expected {
										t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
									}
									return
								}
								if declaration.HasErrors() {
									t.Fatalf("accepted declaration: %v", declaration.Diagnostics)
								}
							}
							if tc.NativeLine > 0 {
								source := rejectedSource(tc, named)
								analysis := sema.Result{}
								if named {
									path := filepath.Join(root, "AsyncCompile"+tc.ID+".cls")
									writeFile(t, path, source)
									writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
									analysis = sema.Analyze(buildIndex(path))
								} else {
									analysis = sema.AnalyzeAnonymous(index, source, api)
								}
								if observed := compileText(analysis); observed != tc.Expected {
									t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
								}
								return
							}
							if named {
								prepareNamed(t)
								if result := runNamed("AsyncProbe", "observed"+tc.ID); result.Status != testreport.StatusPass {
									t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
								}
								return
							}
							source := body(tc, tc.Expected)
							if analysis := sema.AnalyzeAnonymous(index, source, api); analysis.HasErrors() {
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
					}
				})
			}
			for _, tc := range nativeRows {
				t.Run(tc.ID, func(t *testing.T) {
					t.Run("isTest", func(t *testing.T) {
						kind := "exact"
						if len(tc.MatchFields) > 0 {
							kind = "partial"
						}
						counts.TrackKind(t, "@IsTest", kind, 1)
						prepareNamed(t)
						if result := runNamed("AsyncProbe", "observed"+tc.ID); result.Status != testreport.StatusPass {
							t.Fatalf("native control: status %s problem %v", result.Status, result.Problem)
						}
					})
				})
			}
			for _, tc := range data.NativeTests {
				t.Run("drain-"+tc.Method, func(t *testing.T) {
					kind := "exact"
					if tc.Outcome == "Pass" {
						kind = "category"
					}
					counts.TrackKind(t, "@IsTest", kind, 1)
					prepareNamed(t)
					result := runNamed("AsyncA38DrainTest", tc.Method)
					if tc.Outcome == "Pass" {
						if result.Status != testreport.StatusPass {
							t.Fatalf("native drain passed; local status %s problem %v", result.Status, result.Problem)
						}
					} else {
						observed := ""
						if result.Problem != nil {
							typeName := result.Problem.Type
							if !strings.Contains(typeName, ".") {
								typeName = "System." + typeName
							}
							observed = typeName + ": " + result.Problem.Message
						}
						if tc.Outcome != "Fail" || result.Status != testreport.StatusFail || observed != tc.Message {
							t.Fatalf("native drain %s <%s>; local status %s actual <%s> problem %v", tc.Outcome, tc.Message, result.Status, observed, result.Problem)
						}
					}
				})
			}
			// The captured project shadow is a separate source generation. It
			// must not make the original unshadowed R102-R113 rows compile.
			shadowRoot := t.TempDir()
			shadowPaths := append([]string{}, paths...)
			shadowNames := make([]string, 0, len(data.ShadowDeclarations))
			for name := range data.ShadowDeclarations {
				shadowNames = append(shadowNames, name)
			}
			sort.Strings(shadowNames)
			for _, name := range shadowNames {
				path := filepath.Join(shadowRoot, name+".cls")
				writeFile(t, path, data.ShadowDeclarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				shadowPaths = append(shadowPaths, path)
			}
			var shadowSource strings.Builder
			shadowSource.WriteString("@IsTest private class AsyncSignatureShadowProbe {\n")
			for _, tc := range data.ShadowCases {
				fmt.Fprintf(&shadowSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, tc.Expected))
			}
			shadowSource.WriteString("}\n")
			shadowPath := filepath.Join(shadowRoot, "AsyncSignatureShadowProbe.cls")
			writeFile(t, shadowPath, shadowSource.String())
			writeFile(t, shadowPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			shadowPaths = append(shadowPaths, shadowPath)
			shadowIndex := typesys.Build(project.Project{Root: root, ApexFiles: shadowPaths, SourceAPIVersion: api}, gladeschema.Schema{})
			if shadowIndex.HasErrors() {
				t.Fatalf("shadow parser: %v", shadowIndex.Diagnostics)
			}
			if analysis := sema.Analyze(shadowIndex); analysis.HasErrors() {
				t.Fatalf("shadow semantics: %v", analysis.Diagnostics)
			}
			shadowRunner := newConformanceRunner(t, shadowIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true, Org: &org})
			shadowTests := Discover(shadowIndex, Options{SelectedClasses: []string{"AsyncSignatureShadowProbe"}})
			if len(shadowTests) != len(data.ShadowCases) {
				t.Fatalf("shadow discovery: %d expected %d", len(shadowTests), len(data.ShadowCases))
			}
			shadowMethods, shadowMethodErrors := compileTestMethods(shadowTests)
			shadowPrograms, shadowProgramErrors := compileTestInvokePrograms(shadowTests)
			shadowRuntimeMethods := indexTestRuntimeMethods(shadowMethods)
			shadowByName := map[string]TestCase{}
			for _, row := range shadowTests {
				shadowByName[row.MethodName] = row
			}
			shadowCounters := newRunPerfCounters(false)
			for _, tc := range data.ShadowCases {
				t.Run(tc.ID, func(t *testing.T) {
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						source := body(tc, tc.Expected)
						if analysis := sema.AnalyzeAnonymous(shadowIndex, source, api); analysis.HasErrors() {
							t.Fatalf("shadow anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := shadowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						counts.Track(t, "@IsTest", 1)
						row, ok := shadowByName["observed"+tc.ID]
						if !ok {
							t.Fatalf("missing named shadow row %s", tc.ID)
						}
						key := testCaseKey(row)
						seed := shadowRunner.org.CloneRuntimeOrg()
						initializeTestOrg(&seed)
						result := runCase(context.Background(), row, shadowMethods[key], shadowRuntimeMethods[testMethodSourceKey(row.ClassName, row.File)], shadowMethodErrors[key], shadowPrograms[key], shadowProgramErrors[key], shadowRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, shadowCounters)
						if result.Status != testreport.StatusPass {
							t.Fatalf("named shadow row: status %s problem %v", result.Status, result.Problem)
						}
					})
				})
			}
		})
	}
}
