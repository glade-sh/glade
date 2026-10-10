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
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Every layout has its own native answer. Runtime rows use the shared API base;
// compiler rejections stay at the semantic boundary, as in A05 conformance.
func TestNamespaceAccessibleEnclosingOrgConformance(t *testing.T) {
	type nativeRow struct {
		ID, APIVersion, Route, Name, Source, Expected string
	}
	var data struct {
		Rows []nativeRow
	}
	raw, err := os.ReadFile("testdata/conformance/namespace_accessible_enclosing.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 52 {
		t.Fatalf("native row count = %d, want 52", len(data.Rows))
	}
	seen := map[string]bool{}
	for _, row := range data.Rows {
		key := row.APIVersion + "/" + row.Route + "/" + row.ID
		if seen[key] || (row.Route != "anonymous" && row.Name == "") || row.Source == "" || row.Expected == "" || row.Expected == "?" {
			t.Fatalf("invalid native row: %#v", row)
		}
		seen[key] = true
	}
	for _, api := range []string{"58.0", "62.0", "64.0", "67.0"} {
		for _, id := range strings.Fields("C001 C002 C003 C004") {
			if !seen[api+"/named/"+id] {
				t.Fatalf("missing native named observation: %s/%s", api, id)
			}
		}
		if api == "62.0" || api == "67.0" {
			for _, route := range []string{"anonymous", "isTest", "isTestType"} {
				for _, id := range strings.Fields("C001 C002 C003 C004 C005 C006") {
					if !seen[api+"/"+route+"/"+id] {
						t.Fatalf("missing native route observation: %s/%s/%s", api, route, id)
					}
				}
			}
		}
	}

	observation := func(diagnostics []diagnostic.Diagnostic, withLine bool) string {
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			message := d.NativeMessage
			if message == "" {
				message = d.Message
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
	const helper = `public class P {
 public String expectedText;
 public void out(String id,Object value){
  String observedText=String.valueOf(value);
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	const capturedHelper = `class P { public List<String> rows = new List<String>();
  public void out(String id, Object v) { rows.add('P|' + id + '|' + String.valueOf(v)); }
  public void err(String id, Exception e) { rows.add('P|' + id + '|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
}
P pq = new P();
`
	const capturedTail = "class FamilyProbeTransportException extends Exception {}\nthrow new FamilyProbeTransportException('GLADE_FAMILY_ROWS|' + JSON.serialize(pq.rows));\n"
	for _, api := range []string{"58.0", "62.0", "64.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "annotations", api, "namespace")
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
					paths = append(paths, path)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			}
			baseIndex := build(map[string]string{"P": helper})
			if analysis := sema.Analyze(baseIndex); analysis.HasErrors() {
				t.Fatalf("helper semantics: %#v", analysis.Diagnostics)
			}
			org := orgFromIndex(baseIndex)
			org.APIVersion = api
			runner := newConformanceRunner(t, baseIndex, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range data.Rows {
				if row.APIVersion != api {
					continue
				}
				kind := "exact"
				if row.Route == "named" && row.Expected == "compiled" {
					kind = "category"
				}
				counts.RunKind(t, row.Route, row.ID, row.Route+"/"+row.ID, kind, func(t *testing.T) {
					if row.Route == "anonymous" {
						if !strings.HasPrefix(row.Source, capturedHelper) || !strings.HasSuffix(row.Source, capturedTail) {
							t.Fatal("captured anonymous transport changed")
						}
						// Replace only transport framing. The row body and its native
						// line numbers remain byte-identical, starting at line six.
						body := strings.TrimSuffix(strings.TrimPrefix(row.Source, capturedHelper), capturedTail)
						code := strings.Repeat("\n", 4) + "P pq=new P();pq.expectedText=" + conformanceApexString(row.Expected) + ";\n" + body
						parsed := apexast.NewParser().ParseSource("NamespaceAnonymous.cls", code)
						if (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
							if actual := observation(parsed.Diagnostics, true); actual != row.Expected {
								t.Fatalf("native expected <%s>, parser actual <%s>", row.Expected, actual)
							}
							return
						}
						sources := map[string]string{"P": helper}
						masked := []byte(code)
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
						}
						index := sema.WithAnonymousDeclarationContext(build(sources))
						analysis := sema.AnalyzeAnonymousDeclarations(index)
						if !analysis.HasErrors() {
							analysis = sema.AnalyzeAnonymous(index, string(masked), api)
						}
						actual := observation(analysis.Diagnostics, true)
						if actual != row.Expected {
							t.Fatalf("native expected <%s>, anonymous actual <%s>", row.Expected, actual)
						}
						if analysis.HasErrors() {
							return
						}
						program, err := vm.CompileAnonymousWithOptions(string(masked), vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						rowRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
						if _, err := rowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
						return
					}
					index := build(map[string]string{"P": helper, row.Name: row.Source})
					analysis := sema.Analyze(index)
					actual := observation(analysis.Diagnostics, false)
					if actual != row.Expected {
						t.Fatalf("native expected <%s>, %s actual <%s>", row.Expected, row.Route, actual)
					}
					if analysis.HasErrors() || row.Route == "named" {
						return
					}
					// The native test executes observed() and deliberately fails its
					// terminal assertion to frame observedText. Replace only that
					// transport assertion with the exact-text conformance assertion.
					terminal := "System.assert(false,'P|" + row.ID + "|'+observedText);"
					if strings.Count(row.Source, terminal) != 1 {
						t.Fatal("missing captured native test observation")
					}
					assertion := "String expectedText=" + conformanceApexString(row.Expected) + ";System.assert(expectedText.equals(observedText)," + conformanceApexString(row.ID+" expected <"+row.Expected+"> actual <") + "+observedText+'>');"
					source := strings.Replace(row.Source, terminal, assertion, 1)
					executionIndex := build(map[string]string{"P": helper, row.Name: source})
					if result := sema.Analyze(executionIndex); result.HasErrors() {
						t.Fatalf("test assertion semantics: %#v", result.Diagnostics)
					}
					cases := Discover(executionIndex, Options{SelectedClasses: []string{row.Name}})
					if len(cases) != 1 || cases[0].MethodName != "observed" {
						t.Fatalf("missing or ambiguous captured observed(): %#v", cases)
					}
					methods, methodErrors := compileTestMethods(cases)
					programs, programErrors := compileTestInvokePrograms(cases)
					key := testCaseKey(cases[0])
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("test lowering: %v %v", methodErrors[key], programErrors[key])
					}
					rowRunner := newConformanceRunner(t, executionIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
					machine := rowRunner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					if _, err := machine.ExecuteInClass(programs[key], cases[0].ClassName); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
