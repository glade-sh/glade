package apextest

import (
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
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned anonymous and named compiler observations at both API endpoints. The
// linked runtime is shared once per API; each row executes on a private clone.
func TestAnnotationsOrgConformance(t *testing.T) {
	type namedCompilation struct{ Name, Source, Expected string }
	type familyCase struct {
		ID, Group, Code, Expected, FloorExpected string
		Compile, NativeNull                      bool
		NamedCompilations                        map[string]namedCompilation
	}
	var data struct {
		APIVersions     []string `json:"apiVersions"`
		Declarations    map[string]string
		Cases, Controls []familyCase
		TicketRows      map[string][]string
	}
	raw, err := os.ReadFile("testdata/conformance/annotations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 294 || len(data.Controls) != 49 || strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.TicketRows) != 0 {
		t.Fatalf("oracle shape: cases=%d controls=%d APIs=%v tickets=%v", len(data.Cases), len(data.Controls), data.APIVersions, data.TicketRows)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ID == "" || row.Code == "" || seen[row.ID] || row.Expected != row.FloorExpected {
			t.Fatalf("invalid or differing oracle row: %#v", row)
		}
		seen[row.ID] = true
		if row.Compile {
			for _, api := range data.APIVersions {
				native, ok := row.NamedCompilations[api]
				if !ok || native.Name == "" || native.Source == "" || native.Expected == "" {
					t.Fatalf("%s missing named compiler observation at %s", row.ID, api)
				}
			}
		}
	}
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object value){
  String observedText=''+String.valueOf(value);
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  if(expectedNull){
   System.assert(value==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	copySources := func() map[string]string {
		sources := map[string]string{"P": helper}
		for name, source := range data.Declarations {
			sources[name] = source
		}
		return sources
	}
	configuration := func(row familyCase, expectedText string) string {
		text := "pq.expectedText=" + conformanceApexString(expectedText) + ";"
		if row.NativeNull {
			text += "pq.expectedNull=true;"
		}
		return text
	}
	body := func(row familyCase, code string) string {
		prefix := "P pq=new P();" + configuration(row, row.Expected) + "\n"
		if row.Compile {
			return prefix + code
		}
		return prefix + "try {Object r; " + code + " pq.out('" + row.ID + "',r);}catch(Exception e){pq.err('" + row.ID + "',e);}"
	}
	observation := func(diagnostics []diagnostic.Diagnostic, withLine bool) string {
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			message := d.Message
			if d.NativeMessage != "" {
				message = d.NativeMessage
			}
			line := 0
			if d.Range != nil {
				line = d.Range.Start.Line
			}
			if d.NativeLine != nil {
				line = *d.NativeLine
			}
			if withLine && line != 0 {
				return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
			}
			return "COMPILE_ERROR\t" + message
		}
		return "compiled"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "annotations", api, "")
			build := func(sources map[string]string) typesys.Index {
				t.Helper()
				root := t.TempDir()
				names := make([]string, 0, len(sources))
				for name := range sources {
					names = append(names, name)
				}
				sort.Strings(names)
				paths := make([]string, 0, len(names))
				for _, name := range names {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, sources[name])
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, gladeschema.Schema{})
			}
			index := sema.WithAnonymousDeclarationContext(build(copySources()))
			if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
				t.Fatalf("helper semantics: %v", analysis.Diagnostics)
			}
			// J009 needs only standard Account metadata, built locally for CI.
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			compileNamed := func(index typesys.Index, name string) func(*testing.T, string) {
				t.Helper()
				cases := Discover(index, Options{SelectedClasses: []string{name}})
				if len(cases) == 0 {
					t.Fatalf("no named methods in %s", name)
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				byName := map[string]TestCase{}
				for _, testCase := range cases {
					key := testCaseKey(testCase)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("%s named lowering: %v %v", key, methodErrors[key], programErrors[key])
					}
					byName[testCase.MethodName] = testCase
				}
				namedRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
				return func(t *testing.T, methodName string) {
					t.Helper()
					testCase, ok := byName[methodName]
					if !ok {
						t.Fatalf("missing named method %s.%s", name, methodName)
					}
					key := testCaseKey(testCase)
					machine := namedRunner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					if _, err := machine.ExecuteInClass(programs[key], testCase.ClassName); err != nil {
						t.Fatal(err)
					}
				}
			}
			// Ordinary value rows share one named compilation. Source declaration
			// rows use isolated overlays so their names and bodies stay unchanged.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class AnnotationsRuntime {\n")
			for _, row := range rows {
				if !row.Compile {
					fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row, row.Code))
				}
			}
			namedSource.WriteString("}\n")
			var invokeRuntime func(*testing.T, string)
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					counts.Run(t, "anonymous", row.ID, "anonymous", func(t *testing.T) {
						rowIndex, code := index, body(row, row.Code)
						rowRunner := runner
						if row.Compile {
							// The native helper occupies five lines. Keep the row at
							// line six and configure our already-linked helper inline.
							code = "P pq=new P();" + configuration(row, row.Expected) + strings.Repeat("\n", 5) + row.Code
							parsed := apexast.NewParser().ParseSource("AnnotationsAnonymous.cls", code)
							if (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
								actual := observation(parsed.Diagnostics, true)
								if actual != row.Expected {
									t.Fatalf("expected <%s> actual <%s>", row.Expected, actual)
								}
								return
							}
							sources := copySources()
							masked := []byte(code)
							declarations := 0
							for _, declaration := range parsed.Declarations {
								if declaration.Kind != apexast.DeclarationClass && declaration.Kind != apexast.DeclarationInterface && declaration.Kind != apexast.DeclarationEnum {
									continue
								}
								start, end := declaration.Range.Start.Offset, declaration.Range.End.Offset
								prefix := []byte(code[:start])
								for i := range prefix {
									if prefix[i] != '\n' && prefix[i] != '\r' {
										prefix[i] = ' '
									}
								}
								sources[declaration.Name] = string(prefix) + code[start:end]
								for i := start; i < end; i++ {
									if masked[i] != '\n' && masked[i] != '\r' {
										masked[i] = ' '
									}
								}
								declarations++
							}
							if declarations > 0 {
								rowIndex = sema.WithAnonymousDeclarationContext(build(sources))
								analysis := sema.AnalyzeAnonymousDeclarations(rowIndex)
								if analysis.HasErrors() {
									actual := observation(analysis.Diagnostics, true)
									if actual != row.Expected {
										t.Fatalf("expected <%s> actual <%s>", row.Expected, actual)
									}
									return
								}
								rowRunner = newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
							}
							code = string(masked)
						}
						if analysis := sema.AnalyzeAnonymous(rowIndex, code, api); analysis.HasErrors() {
							actual := observation(analysis.Diagnostics, true)
							if actual != row.Expected {
								t.Fatalf("expected <%s> actual <%s>", row.Expected, actual)
							}
							return
						}
						if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
							t.Fatal("native rejection was accepted")
						}
						program, err := vm.CompileAnonymousWithOptions(code, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err := rowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					counts.Run(t, "isTest", row.ID, "isTest", func(t *testing.T) {
						if !row.Compile {
							if invokeRuntime == nil {
								sources := copySources()
								sources["AnnotationsRuntime"] = namedSource.String()
								namedIndex := build(sources)
								if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
									t.Fatalf("named runtime semantics: %v", analysis.Diagnostics)
								}
								invokeRuntime = compileNamed(namedIndex, "AnnotationsRuntime")
							}
							invokeRuntime(t, "observed"+row.ID)
							return
						}
						native := row.NamedCompilations[api]
						sources := copySources()
						sources[native.Name] = native.Source
						namedIndex := build(sources)
						actual := observation(sema.Analyze(namedIndex).Diagnostics, false)
						if actual != native.Expected {
							t.Fatalf("named compiler expected <%s> actual <%s>", native.Expected, actual)
						}
						if native.Expected != "compiled" {
							return
						}
						// Keep the captured body; replace only its observation helper
						// and configure the exact-text assertion on the same line.
						// S002 admits the named global type but rejects it anonymously.
						namedExpected := row.Expected
						if strings.HasPrefix(namedExpected, "COMPILE_ERROR\t") {
							namedExpected = native.Expected
						}
						source := strings.Replace(native.Source, "P pq=new P();", "P pq=new P();"+configuration(row, namedExpected), 1)
						const capturedHelper = "private class P {public void out(String id,Object value){} public void err(String id,Exception e){}}"
						if !strings.Contains(source, capturedHelper) {
							t.Fatal("missing captured observation helper")
						}
						source = strings.Replace(source, capturedHelper, strings.Replace(helper, "public class P", "private class P", 1), 1)
						sources[native.Name] = source
						executionIndex := build(sources)
						if analysis := sema.Analyze(executionIndex); analysis.HasErrors() {
							t.Fatalf("named assertion semantics: %v", analysis.Diagnostics)
						}
						compileNamed(executionIndex, native.Name)(t, "observed")
					})
				})
			}
		})
	}
}
