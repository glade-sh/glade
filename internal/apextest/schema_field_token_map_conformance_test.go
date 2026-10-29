package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// All 44 synthetic rows were captured at API 62/67/68 on both routes.
// One expected value per row serves anonymous and named execution. Hash and
// diagnostic wording carries retain their native values and stay out of exact
// match counts; their execution/rejection is still checked on both routes.
func TestSchemaFieldTokenMapOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected                                  string
		CarryOwner, CarryReason                             string
		NativeDiagnostic, DiagnosticOwner, DiagnosticReason string
		Compile                                             bool
	}
	var data struct {
		APIVersions []string `json:"apiVersions"`
		Routes      []string
		Cases       []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/schema_field_token_map.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 44 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "anonymous,isTest" {
		t.Fatalf("oracle: %d rows, APIs %v, routes %v", len(data.Cases), data.APIVersions, data.Routes)
	}
	seen := map[string]bool{}
	accepted, carried, rejected := 0, 0, 0
	for _, tc := range data.Cases {
		if tc.ID == "" || seen[tc.ID] || tc.Code == "" || tc.Expected == "?" || tc.Compile != strings.HasPrefix(tc.Expected, "COMPILE_ERROR\t") {
			t.Fatalf("invalid/repeated row: %#v", tc)
		}
		seen[tc.ID] = true
		if (tc.CarryOwner == "") != (tc.CarryReason == "") || (tc.DiagnosticOwner == "") != (tc.DiagnosticReason == "") {
			t.Fatalf("incomplete carry: %#v", tc)
		}
		if tc.Compile {
			rejected++
			if tc.NativeDiagnostic == "" || tc.NativeDiagnostic != strings.TrimPrefix(tc.Expected, "COMPILE_ERROR\t") || tc.CarryOwner != "" {
				t.Fatalf("invalid rejection: %#v", tc)
			}
		} else {
			accepted++
			if tc.DiagnosticOwner != "" {
				t.Fatalf("diagnostic carry on accepted row: %#v", tc)
			}
			if tc.CarryOwner != "" {
				carried++
			}
		}
	}
	if accepted != 42 || carried != 2 || rejected != 2 {
		t.Fatalf("oracle: accepted %d (carried %d), rejected %d", accepted, carried, rejected)
	}
	assertRejection := func(t *testing.T, tc familyCase, diagnostics []diagnostic.Diagnostic, compileErr error) {
		t.Helper()
		var messages []string
		for _, d := range diagnostics {
			if d.Severity == diagnostic.Error {
				text := d.NativeMessage
				if text == "" {
					text = d.Message
				}
				messages = append(messages, text)
			}
		}
		if len(messages) == 0 && compileErr != nil {
			messages = append(messages, compileErr.Error())
		}
		if len(messages) == 0 {
			t.Fatal("org rejects compilation; Glade accepted")
		}
		observed := strings.Join(messages, " | ")
		if tc.DiagnosticOwner != "" {
			t.Logf("%s diagnostic carried to %s: %s; native <%s> actual <%s>", tc.ID, tc.DiagnosticOwner, tc.DiagnosticReason, tc.NativeDiagnostic, observed)
			return
		}
		if observed != tc.NativeDiagnostic {
			t.Fatalf("native diagnostic <%s> actual <%s>", tc.NativeDiagnostic, observed)
		}
	}
	helper := `public class P {
 public String expectedText;
 public Boolean carry=false;
 public void out(String id,Object value){
  String observedText=String.valueOf(value);
  if(carry){System.debug(observedText);return;}
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){System.assert(false,id+' unexpected '+e.getTypeName()+': '+e.getMessage());}
}`
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + "; pq.carry=" + fmt.Sprint(tc.CarryOwner != "") + ";\n"
		if tc.Compile {
			return prefix + tc.Code
		}
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	logCarry := func(t *testing.T, tc familyCase, debug []string) {
		t.Helper()
		if len(debug) != 1 {
			t.Fatalf("%s carried row did not produce one observation: %q", tc.ID, debug)
		}
		t.Logf("%s carried to %s: %s; native <%s> actual <%s>", tc.ID, tc.CarryOwner, tc.CarryReason, tc.Expected, debug[0])
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "SchemaFieldTokenMap", api)
			root := t.TempDir()
			helperPath := filepath.Join(root, "P.cls")
			writeFile(t, helperPath, helper)
			writeFile(t, helperPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			buildIndex := func(extra string) typesys.Index {
				paths := []string{helperPath}
				if extra != "" {
					paths = append(paths, extra)
				}
				return typesys.Build(project.Project{Root: root, ApexFiles: paths, SourceAPIVersion: api}, gladeschema.Schema{})
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
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class FieldTokenMapConformance {\n")
			for _, tc := range data.Cases {
				if !tc.Compile {
					fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
				}
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "FieldTokenMapConformance.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "FieldTokenMapConformance" || namedResults[id].MethodName != "" || !seen[id] {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if run.Summary().Total != accepted || len(namedResults) != accepted {
				t.Fatalf("named results: %#v, unique %d expected %d: %s", run.Summary(), len(namedResults), accepted, firstRunProblem(run))
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					kind := "exact"
					if tc.CarryOwner != "" || tc.DiagnosticOwner != "" {
						kind = "carried"
					}
					source := body(tc)
					counts.RunKind(t, "anonymous", tc.ID, "anonymous", kind, func(t *testing.T) {
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if tc.Compile {
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
						if tc.CarryOwner != "" {
							logCarry(t, tc, result.Debug)
						}
					})
					counts.RunKind(t, "@IsTest", tc.ID, "isTest", kind, func(t *testing.T) {
						if !tc.Compile {
							result := namedResults[tc.ID]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
							}
							if tc.CarryOwner != "" {
								// As in Compression conformance, invoke the indexed named
								// method to inspect debug omitted by the ordinary report.
								cases := Discover(namedIndex, Options{SelectedClasses: []string{"FieldTokenMapConformance"}, SelectedMethod: "observed" + tc.ID})
								if len(cases) != 1 {
									t.Fatalf("carry method discovery: %d", len(cases))
								}
								methods, methodErrors := compileTestMethods(cases)
								invocations, invocationErrors := compileTestInvokePrograms(cases)
								key := testCaseKey(cases[0])
								if err := methodErrors[key]; err != nil {
									t.Fatal(err)
								}
								if err := invocationErrors[key]; err != nil {
									t.Fatal(err)
								}
								machine := namedRunner.newMachine()
								if err := machine.RegisterMethod(methods[key]); err != nil {
									t.Fatal(err)
								}
								machine.EnableTestContext()
								observed, err := machine.ExecuteInClass(invocations[key], cases[0].ClassName)
								if err != nil {
									t.Fatal(err)
								}
								logCarry(t, tc, observed.Debug)
							}
							return
						}
						path := filepath.Join(root, "FieldTokenMapRejected.cls")
						writeFile(t, path, "@IsTest private class FieldTokenMapRejected { @IsTest static void observed(){\n"+source+"\n} }")
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
