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

// Captured lifecycle observations run without Salesforce or glade-tools in CI.
func TestClassesLifecycleOrgConformance(t *testing.T) {
	type namedCompilation struct{ Name, Source, Expected string }
	type familyCase struct {
		ID, Group, Code, Expected string
		Compile                   bool
		NativeNull                bool
		Owner, Reason             string
		SourceLine                int
		NamedCompilations         map[string]namedCompilation
	}
	var data struct {
		APIVersions           []string `json:"apiVersions"`
		Declarations          map[string]string
		AnonymousDeclarations map[string]string `json:"anonymousDeclarations"`
		Cases, Controls       []familyCase
		NamedCompilerControls []familyCase `json:"namedCompilerControls"`
		Remaining             []familyCase
		CaseRows              map[string][]string `json:"caseRows"`
	}
	raw, err := os.ReadFile("testdata/conformance/classes_lifecycle.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 258 || len(data.Controls) != 31 || len(data.Remaining) != 0 || len(data.NamedCompilerControls) != 6 || len(data.CaseRows["all"]) != 258 || len(data.CaseRows["access-modifiers-and-namespaces"]) != 12 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle shape: cases=%d controls=%d carried=%d versions=%v", len(data.Cases), len(data.Controls), len(data.Remaining), data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ID == "" || row.Code == "" || seen[row.ID] {
			t.Fatalf("invalid assertion row: %#v", row)
		}
		if strings.HasPrefix(row.Expected, "COMPILE_ERROR") && (row.SourceLine < 1 || len(row.NamedCompilations) != 2) {
			t.Fatalf("invalid exact anonymous/named rejection: %#v", row)
		}
		seen[row.ID] = true
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A04" || row.Reason == "" || row.Expected == "" {
			t.Fatalf("invalid carried row: %#v", row)
		}
		seen[row.ID] = true
		// Entire diagnostic differences are carried, never category-only asserts.
		t.Logf("%s carried to %s: %s; native <%s>", row.ID, row.Owner, row.Reason, row.Expected)
	}
	for _, row := range data.NamedCompilerControls {
		if row.ID == "" || seen[row.ID] || len(row.NamedCompilations) != len(data.APIVersions) {
			t.Fatalf("invalid named compiler control: %#v", row)
		}
		for _, api := range data.APIVersions {
			native, captured := row.NamedCompilations[api]
			if !captured || native.Name == "" || native.Source == "" || native.Expected == "" {
				t.Fatalf("missing compiler control %s at %s", row.ID, api)
			}
		}
		seen[row.ID] = true
	}
	for group, ids := range data.CaseRows {
		for _, id := range ids {
			if !seen[id] {
				t.Fatalf("%s row has no disposition: %s", group, id)
			}
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
	body := func(tc familyCase, code string) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + code
		}
		if !strings.Contains(code, ";") {
			code = "r=" + code + ";"
		}
		return prefix + "try {Object r; " + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	declarations := func(code string, rejected bool) (map[string]string, string) {
		parsed := apexast.NewParser().ParseSource("LifecycleAnonymous.cls", code)
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
				t.Fatalf("invalid declaration range: %s", decl.Name)
			}
			out[decl.Name] = code[start:end]
			for i := start; i < end; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
		if !rejected && len(out) > 0 && (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors() {
			t.Fatalf("accepted declaration parser: %v", parsed.Diagnostics)
		}
		return out, string(masked)
	}
	copySources := func(sources map[string]string) map[string]string {
		out := make(map[string]string, len(sources)+1)
		for name, source := range sources {
			out[name] = source
		}
		out["P"] = helper
		return out
	}
	assertUncaught := func(t *testing.T, tc familyCase, message string) {
		t.Helper()
		observedText := "UNCAUGHT|" + message
		if tc.Expected != observedText {
			t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.Expected, observedText)
		}
	}
	compileObservation := func(diagnostics []diagnostic.Diagnostic, withLine bool) string {
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			if !withLine {
				// Tooling compilation reports the bare native message. Strip
				// location/code metadata, then use the product compile gate's
				// rendering without enriching or rewriting the diagnostic.
				d.File, d.Code, d.Range = "", "", nil
				return "COMPILE_ERROR\t" + indexCompileError([]diagnostic.Diagnostic{d}).Error()
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
		return ""
	}
	assertExecution := func(t *testing.T, tc familyCase, execErr error) {
		t.Helper()
		if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
			if execErr == nil {
				t.Fatalf("%s expected uncaught failure", tc.ID)
			}
			assertUncaught(t, tc, execErr.Error())
		} else if execErr != nil {
			t.Fatalf("%s execution: %v", tc.ID, execErr)
		}
	}
	// Compile named methods once for this source set, then invoke only selected
	// rows on private clones of the shared linked API base. Bodies are unchanged.
	compileNamed := func(t *testing.T, index typesys.Index, className string, base *conformanceRunner) func(*testing.T, string) error {
		t.Helper()
		cases := Discover(index, Options{SelectedClasses: []string{className}})
		if len(cases) == 0 {
			t.Fatalf("no named rows in %s", className)
		}
		methods, methodErrors := compileTestMethods(cases)
		programs, programErrors := compileTestInvokePrograms(cases)
		byMethod := make(map[string]TestCase, len(cases))
		for _, tc := range cases {
			key := testCaseKey(tc)
			if methodErrors[key] != nil || programErrors[key] != nil {
				t.Fatalf("%s named compile: %v %v", key, methodErrors[key], programErrors[key])
			}
			if _, exists := byMethod[tc.MethodName]; exists {
				t.Fatalf("repeated named method %s.%s", className, tc.MethodName)
			}
			byMethod[tc.MethodName] = tc
		}
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: base, LinkProject: true})
		return func(t *testing.T, name string) error {
			t.Helper()
			tc, exists := byMethod[name]
			if !exists {
				t.Fatalf("missing named row %s.%s", className, name)
			}
			key := testCaseKey(tc)
			machine := runner.newMachine()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			machine.SetCurrentPageURLNull()
			machine.SetTestSeeAllData(tc.SeeAllData)
			_, err := machine.ExecuteInClass(programs[key], tc.ClassName)
			return err
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "classes and lifecycle", api)
			build := func(root string, sources map[string]string) typesys.Index {
				t.Helper()
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
			for _, tc := range data.NamedCompilerControls {
				t.Run(tc.ID, func(t *testing.T) {
					t.Run("isTestCompiler", func(t *testing.T) {
						kind := "exact"
						if tc.NamedCompilations[api].Expected == "compiled" {
							kind = "category"
						}
						counts.TrackKind(t, "@IsTest", kind, 1)
						native := tc.NamedCompilations[api]
						namedIndex := build(t.TempDir(), map[string]string{native.Name: native.Source})
						diagnostics := sema.Analyze(namedIndex).Diagnostics
						observedText := compileObservation(diagnostics, false)
						if observedText == "" {
							observedText = "compiled"
						}
						if observedText != native.Expected {
							t.Fatalf("%s named compiler expected <%s> actual <%s>", tc.ID, native.Expected, observedText)
						}
					})
				})
			}
			anonymousSources := copySources(data.AnonymousDeclarations)
			index := sema.WithAnonymousDeclarationContext(build(t.TempDir(), anonymousSources))
			if index.HasErrors() {
				t.Fatalf("anonymous helpers: %v", index.Diagnostics)
			}
			if analysis := sema.AnalyzeAnonymousDeclarations(index); analysis.HasErrors() {
				t.Fatalf("anonymous helper semantics: %v", analysis.Diagnostics)
			}
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			// Ordinary runtime rows share one named compilation; declaration rows
			// retain isolated companion classes. Test selection stays at row level.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class ClassesLifecycleProbe {\n")
			grouped := map[string]bool{}
			for _, tc := range rows {
				if tc.Compile || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				grouped[tc.ID] = true
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, tc.Code))
			}
			namedSource.WriteString("}\n")
			var namedInvoke func(*testing.T, string) error
			prepareNamed := func(t *testing.T) {
				t.Helper()
				if namedInvoke != nil {
					return
				}
				sources := copySources(data.Declarations)
				sources["ClassesLifecycleProbe"] = namedSource.String()
				namedIndex := build(t.TempDir(), sources)
				if namedIndex.HasErrors() {
					t.Fatalf("named parser: %v", namedIndex.Diagnostics)
				}
				if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
					t.Fatalf("named semantics: %v", analysis.Diagnostics)
				}
				cases := Discover(namedIndex, Options{SelectedClasses: []string{"ClassesLifecycleProbe"}})
				if len(cases) != len(grouped) {
					t.Fatalf("named rows: %d expected %d", len(cases), len(grouped))
				}
				for _, tc := range cases {
					if !grouped[strings.TrimPrefix(tc.MethodName, "observed")] {
						t.Fatalf("unexpected named row: %s", tc.MethodName)
					}
				}
				namedInvoke = compileNamed(t, namedIndex, "ClassesLifecycleProbe", anonymousRunner)
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					decls, code := map[string]string{}, tc.Code
					if tc.Compile {
						decls, code = declarations(tc.Code, strings.HasPrefix(tc.Expected, "COMPILE_ERROR"))
					}
					rowIndex := index
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					var declarationDiagnostics []diagnostic.Diagnostic
					if len(decls) > 0 {
						sources := copySources(anonymousSources)
						for name, source := range decls {
							if rejected {
								source = strings.Repeat("\n", tc.SourceLine-1) + source
							}
							sources[name] = source
						}
						rowIndex = sema.WithAnonymousDeclarationContext(build(t.TempDir(), sources))
						if rowIndex.HasErrors() && !rejected {
							t.Fatalf("accepted declarations: %v", rowIndex.Diagnostics)
						}
						analysis := sema.AnalyzeAnonymousDeclarations(rowIndex)
						if rejected {
							declarationDiagnostics = analysis.Diagnostics
						} else if analysis.HasErrors() {
							t.Fatalf("accepted declaration semantics: %v", analysis.Diagnostics)
						}
					}
					source := body(tc, code)
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						if rejected {
							// Companion helper files replace the native capture's
							// prelude. Preserve its row line for exact diagnostics.
							observedText := compileObservation(declarationDiagnostics, true)
							if observedText == "" {
								if tc.SourceLine < 2 {
									t.Fatal("native declaration rejection was accepted")
								}
								anonymousSource := strings.Repeat("\n", tc.SourceLine-2) + source
								analysis := sema.AnalyzeAnonymous(rowIndex, anonymousSource, api)
								observedText = compileObservation(analysis.Diagnostics, true)
							}
							if observedText != tc.Expected {
								t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.Expected, observedText)
							}
							return
						}
						if analysis := sema.AnalyzeAnonymous(rowIndex, source, api); analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						runner := anonymousRunner
						if len(decls) > 0 {
							runner = newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true})
						}
						_, execErr := runner.execute(program)
						assertExecution(t, tc, execErr)
					})
					t.Run("isTest", func(t *testing.T) {
						counts.Track(t, "@IsTest", 1)
						if native, captured := tc.NamedCompilations[api]; captured {
							sources := copySources(data.Declarations)
							sources[native.Name] = native.Source
							namedIndex := build(t.TempDir(), sources)
							diagnostics := sema.Analyze(namedIndex).Diagnostics
							if tc.ID == "C032" || tc.ID == "C033" {
								for _, d := range diagnostics {
									if d.Severity == diagnostic.Error {
										observedText := "COMPILE_ERROR\t" + d.Message
										if observedText != native.Expected {
											t.Fatalf("%s ordinary analysis expected <%s> actual <%s>", tc.ID, native.Expected, observedText)
										}
										break
									}
								}
							}
							observedText := compileObservation(diagnostics, false)
							if observedText == "" {
								observedText = "compiled"
							}
							if observedText != native.Expected {
								t.Fatalf("%s named compiler expected <%s> actual <%s>", tc.ID, native.Expected, observedText)
							}
							if native.Expected != "compiled" {
								return
							}
							invoke := compileNamed(t, namedIndex, native.Name, anonymousRunner)
							assertExecution(t, tc, invoke(t, "observed"))
							return
						}
						if grouped[tc.ID] {
							prepareNamed(t)
							assertExecution(t, tc, namedInvoke(t, "observed"+tc.ID))
							return
						}
						sources := copySources(data.Declarations)
						for name, declaration := range decls {
							// C049's private anonymous class is an inner type. Its
							// named companion needs top-level visibility instead.
							declaration = strings.TrimSpace(declaration)
							declaration = strings.TrimPrefix(declaration, "private ")
							if !strings.HasPrefix(strings.TrimSpace(declaration), "public ") && !strings.HasPrefix(strings.TrimSpace(declaration), "global ") {
								declaration = "public " + declaration
							}
							sources[name] = declaration
						}
						sources["ClassesLifecycleIsolated"] = "@IsTest private class ClassesLifecycleIsolated { @IsTest static void observed(){\n" + source + "\n} }"
						isolated := build(t.TempDir(), sources)
						if isolated.HasErrors() {
							t.Fatalf("named declaration parser: %v", isolated.Diagnostics)
						}
						if analysis := sema.Analyze(isolated); analysis.HasErrors() {
							t.Fatalf("named declaration semantics: %v", analysis.Diagnostics)
						}
						invoke := compileNamed(t, isolated, "ClassesLifecycleIsolated", anonymousRunner)
						assertExecution(t, tc, invoke(t, "observed"))
					})
				})
			}
		})
	}
}
