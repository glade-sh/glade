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

// Compression observations at API 62 and 67. CI needs no org or probe tools.
func TestCompressionOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected                string
		NativeDiagnostic, DiagnosticOwner string
		Compile, NativeNull               bool
		DebugExpected                     []string
	}
	var data struct {
		APIVersions     []string `json:"apiVersions"`
		Declarations    map[string]string
		Cases, Controls []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/compression.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 239 || len(data.Controls) != 24 {
		t.Fatalf("oracle rows: %d + %d", len(data.Cases), len(data.Controls))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	assertDebug := func(t *testing.T, tc familyCase, observed []string) {
		t.Helper()
		if tc.DebugExpected == nil {
			return
		}
		if len(observed) != len(tc.DebugExpected) {
			t.Fatalf("%s debug expected %q actual %q", tc.ID, tc.DebugExpected, observed)
		}
		for i, expectedText := range tc.DebugExpected {
			if expectedText != observed[i] {
				t.Fatalf("%s debug %d expected <%s> actual <%s>", tc.ID, i, expectedText, observed[i])
			}
		}
	}
	assertRejection := func(t *testing.T, tc familyCase, diagnostics []diagnostic.Diagnostic, compileErr error) {
		t.Helper()
		var messages []string
		for _, d := range diagnostics {
			if d.Severity == diagnostic.Error {
				messages = append(messages, d.Message)
			}
		}
		if len(messages) == 0 && compileErr != nil {
			messages = append(messages, compileErr.Error())
		}
		if len(messages) == 0 {
			t.Fatal("org rejects compilation; Glade accepted")
		}
		if tc.NativeDiagnostic == "" {
			t.Fatal("missing native diagnostic text")
		}
		observedText := strings.Join(messages, " | ")
		if tc.DiagnosticOwner != "" {
			// Carried text differences are visible in the review log, not asserted.
			t.Logf("%s diagnostic text carried to %s: native <%s> glade <%s>", tc.ID, tc.DiagnosticOwner, tc.NativeDiagnostic, observedText)
			return
		}
		if observedText != tc.NativeDiagnostic {
			t.Fatalf("%s diagnostic expected <%s> actual <%s>", tc.ID, tc.NativeDiagnostic, observedText)
		}
	}
	// Inline capture helpers need public visibility when written as .cls files.
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull;
 public void out(String id,Object v){
  if(expectedNull){System.assert(v==null,id+' expected raw null actual <'+String.valueOf(v)+'>');return;}
  String observedText=String.valueOf(v);
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  if(expectedNull){System.assert(false,id+' expected raw null actual <'+observedText+'>');return;}
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + "; pq.expectedNull=" + fmt.Sprint(tc.NativeNull) + ";\n"
		if tc.Compile {
			return prefix + tc.Code
		}
		code := tc.Code
		if !strings.Contains(code, ";") {
			code = "r=" + code + ";"
		}
		return prefix + "try {Object r; " + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "compression", api)
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
				return typesys.Build(project.Project{Root: root, ApexFiles: files}, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if a := sema.Analyze(index); a.HasErrors() {
				t.Fatalf("helper semantics: %v", a.Diagnostics)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			// All accepted rows share one named-class compilation, but each @IsTest
			// method gets its own runner state. Rejected rows retain isolated compilation.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class CompressionProbe {\n")
			accepted := 0
			for _, tc := range rows {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "CompressionProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if a := sema.Analyze(namedIndex); a.HasErrors() {
				t.Fatalf("named semantics: %v", a.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{LinkProject: true})
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "CompressionProbe" || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if run.Summary().Total != accepted || len(namedResults) != accepted {
				t.Fatalf("named results: %#v, unique%d expected%d: %s", run.Summary(), len(namedResults), accepted, firstRunProblem(run))
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if rejected && tc.DiagnosticOwner != "" {
						// Rejection is asserted, but the diagnostic text is carried.
						// Keep it out of the exact-text subtotal on both routes.
						kind = "carried"
					}
					t.Run("anonymous", func(t *testing.T) {
						counts.TrackKind(t, "anonymous", kind, 1)
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if rejected {
							assertRejection(t, tc, analysis.Diagnostics, compileErr)
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						result, err := runner.execute(program)
						if err != nil {
							t.Fatal(err)
						}
						assertDebug(t, tc, result.Debug)
					})
					t.Run("isTest", func(t *testing.T) {
						counts.TrackKind(t, "@IsTest", kind, 1)
						if !rejected {
							result := namedResults[tc.ID]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status%s problem%v", result.Status, result.Problem)
							}
							if tc.DebugExpected != nil {
								// Runner reports omit debug output. Invoke the indexed @IsTest
								// method in class/test context to inspect its emitted messages.
								cases := Discover(namedIndex, Options{SelectedClasses: []string{"CompressionProbe"}, SelectedMethod: "observed" + tc.ID})
								if len(cases) != 1 {
									t.Fatalf("%s debug method discovery: %d", tc.ID, len(cases))
								}
								testMethods, methodErrors := compileTestMethods(cases)
								invocations, invocationErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if err := methodErrors[key]; err != nil {
									t.Fatal(err)
								}
								if err := invocationErrors[key]; err != nil {
									t.Fatal(err)
								}
								machine := namedRunner.newMachine()
								if err := machine.RegisterMethod(testMethods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								observed, err := machine.ExecuteInClass(invocations[key], cases[0].ClassName)
								if err != nil {
									t.Fatal(err)
								}
								assertDebug(t, tc, observed.Debug)
							}
							return
						}
						path := filepath.Join(root, "CompressionRejected.cls")
						writeFile(t, path, "@IsTest private class CompressionRejected { @IsTest static void observed(){"+source+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						rejectedIndex := buildIndex(path)
						analysis := sema.Analyze(rejectedIndex)
						var compileErr error
						if !rejectedIndex.HasErrors() && !analysis.HasErrors() {
							r := Run(rejectedIndex, Options{NoDiskCache: true, Parallelism: 1})
							if r.Summary().CompileErrors == 0 {
								t.Fatalf("org rejects compilation; named path accepted: %#v %s", r.Summary(), firstRunProblem(r))
							}
							compileErr = fmt.Errorf("%s", firstRunProblem(r))
						}
						diagnostics := append(append([]diagnostic.Diagnostic{}, rejectedIndex.Diagnostics...), analysis.Diagnostics...)
						assertRejection(t, tc, diagnostics, compileErr)
					})
				})
			}
		})
	}
}
