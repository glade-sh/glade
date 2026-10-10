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
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// The exported native observations are sufficient for credential-free CI.
func TestExceptionsOrgConformance(t *testing.T) {
	type namedCompilation struct{ Name, Source, Expected string }
	type familyCase struct {
		ID, Group, Code, Expected string
		Compile, NativeNull       bool
		SourceLine                int
		NamedCompilations         map[string]namedCompilation
		Owner, Reason             string
	}
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		OrgObjects             []string `json:"orgObjects"`
		Declarations           map[string]string
		DependencyDeclarations []string
		Cases, Controls        []familyCase
		Remaining              []familyCase
		TicketRows             map[string][]string
	}
	raw, err := os.ReadFile("testdata/conformance/exceptions.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 256 || len(data.Controls) != 48 || len(data.Remaining) != 0 || len(data.TicketRows) != 0 || strings.Join(data.APIVersions, ",") != "62.0,67.0" || strings.Join(data.OrgObjects, ",") != "Account" || strings.Join(data.DependencyDeclarations, ",") != "A06DependencyBaseException,A06DependencyMidException" {
		t.Fatalf("oracle shape: cases=%d controls=%d carried=%d versions=%v", len(data.Cases), len(data.Controls), len(data.Remaining), data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ID == "" || row.Code == "" || seen[row.ID] {
			t.Fatalf("invalid assertion row: %#v", row)
		}
		if strings.HasPrefix(row.Expected, "COMPILE_ERROR") && row.SourceLine < 1 {
			t.Fatalf("missing rejection line: %#v", row)
		}
		if row.Compile || strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
			for _, api := range data.APIVersions {
				capture, ok := row.NamedCompilations[api]
				if !ok || capture.Name == "" || capture.Source == "" || capture.Expected == "" {
					t.Fatalf("missing native named compilation: %s/%s", row.ID, api)
				}
			}
		}
		seen[row.ID] = true
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Owner == "A06" || row.Reason == "" {
			t.Fatalf("invalid carry: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("%s carried to %s: %s; native <%s>", row.ID, row.Owner, row.Reason, row.Expected)
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
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase, code string) string {
		prefix := "P pq=new P();pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;"
		}
		if tc.Compile {
			return prefix + code
		}
		return prefix + "try{Object r;" + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	compileObservation := func(diagnostics []diagnostic.Diagnostic, withLine bool) string {
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
			if withLine && line > 0 {
				return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
			}
			return "COMPILE_ERROR\t" + message
		}
		return "compiled"
	}
	declarations := func(code string) (map[string]string, string) {
		parsed := apexast.NewParser().ParseSource("ExceptionsAnonymous.cls", code)
		out := map[string]string{}
		masked := []byte(code)
		for _, decl := range parsed.Declarations {
			if decl.Kind != apexast.DeclarationClass {
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
		return out, string(masked)
	}
	copySources := func() map[string]string {
		out := map[string]string{"P": helper}
		for name, source := range data.Declarations {
			out[name] = source
		}
		return out
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "exceptions", api)
			build := func(sources map[string]string) typesys.Index {
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
				index := typesys.Build(project.Project{Root: root, ApexFiles: paths}, gladeschema.Schema{})
				for i := range index.Types {
					for _, name := range data.DependencyDeclarations {
						if index.Types[i].Name == name {
							// The oracle loads these named base classes separately.
							// Preserve their ancestry as dependency artifact metadata.
							index.Types[i].Dependency = true
							index.Types[i].Artifact = true
						}
					}
				}
				return index
			}
			index := build(copySources())
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatalf("helper semantics: %v", analysis.Diagnostics)
			}
			// R143-R148 require standard Account schema and an empty record set.
			// Build that state from the export, without a probe project or org auth.
			org := storage.NewOrgState()
			org.APIVersion = api
			for _, objectName := range data.OrgObjects {
				storage.EnsureStandardObject(&org, objectName)
			}
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			compileNamed := func(t *testing.T, namedIndex typesys.Index, className string) func(*testing.T, string) {
				t.Helper()
				cases := Discover(namedIndex, Options{SelectedClasses: []string{className}})
				if len(cases) == 0 {
					t.Fatalf("no named rows: %s", className)
				}
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				byMethod := make(map[string]TestCase, len(cases))
				for _, tc := range cases {
					key := testCaseKey(tc)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("%s named compile: %v %v", key, methodErrors[key], programErrors[key])
					}
					byMethod[tc.MethodName] = tc
				}
				runner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
				return func(t *testing.T, method string) {
					t.Helper()
					tc, ok := byMethod[method]
					if !ok {
						t.Fatalf("missing named row: %s.%s", className, method)
					}
					key := testCaseKey(tc)
					machine := runner.newMachine()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					machine.SetCurrentPageURLNull()
					machine.SetTestSeeAllData(tc.SeeAllData)
					if _, err := machine.ExecuteInClass(programs[key], tc.ClassName); err != nil {
						t.Fatal(err)
					}
				}
			}
			var groupedSource strings.Builder
			groupedSource.WriteString("@IsTest private class ExceptionsProbe {\n")
			for _, row := range rows {
				if !row.Compile && !strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
					fmt.Fprintf(&groupedSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row, row.Code))
				}
			}
			groupedSource.WriteString("}\n")
			var namedInvoke func(*testing.T, string)
			prepareNamed := func(t *testing.T) {
				t.Helper()
				if namedInvoke != nil {
					return
				}
				sources := copySources()
				sources["ExceptionsProbe"] = groupedSource.String()
				namedIndex := build(sources)
				if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
					t.Fatalf("named semantics: %v", analysis.Diagnostics)
				}
				namedInvoke = compileNamed(t, namedIndex, "ExceptionsProbe")
			}
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					rejected := strings.HasPrefix(row.Expected, "COMPILE_ERROR")
					decls, code := map[string]string{}, row.Code
					if row.Compile {
						decls, code = declarations(row.Code)
					}
					rowIndex := index
					var declarationDiagnostics []diagnostic.Diagnostic
					if len(decls) > 0 {
						sources := copySources()
						for name, source := range decls {
							if rejected {
								source = strings.Repeat("\n", row.SourceLine-1) + source
							}
							sources[name] = source
						}
						namedDeclarations := build(sources)
						rowIndex = sema.WithAnonymousDeclarationContext(namedDeclarations)
						transient := typesys.Index{}
						for i, typ := range rowIndex.Types {
							if _, declared := decls[strings.TrimSuffix(filepath.Base(typ.File), ".cls")]; declared {
								transient.Types = append(transient.Types, typ)
							} else {
								// Loaded named helpers retain their original nesting.
								rowIndex.Types[i] = namedDeclarations.Types[i]
							}
						}
						analysis := sema.AnalyzeAnonymousDeclarationsInContext(rowIndex, transient)
						declarationDiagnostics = analysis.Diagnostics
						if !rejected && analysis.HasErrors() {
							t.Fatalf("anonymous declaration semantics: %v", analysis.Diagnostics)
						}
					}
					counts.run(t, "anonymous", "anonymous", "exact", func(t *testing.T) {
						source := body(row, code)
						if rejected {
							observed := compileObservation(declarationDiagnostics, true)
							if observed == "compiled" {
								if row.SourceLine < 2 {
									t.Fatal("native declaration rejection was accepted")
								}
								source = strings.Repeat("\n", row.SourceLine-2) + source
								observed = compileObservation(sema.AnalyzeAnonymous(rowIndex, source, api).Diagnostics, true)
							}
							if observed != row.Expected {
								t.Fatalf("%s expected <%s> actual <%s>", row.ID, row.Expected, observed)
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
						runner := base
						if len(decls) > 0 {
							runner = newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: base, LinkProject: true, Org: &org})
						}
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					counts.run(t, "isTest", "@IsTest", "exact", func(t *testing.T) {
						if native, captured := row.NamedCompilations[api]; captured {
							sources := copySources()
							sources[native.Name] = native.Source
							namedIndex := build(sources)
							analysis := sema.Analyze(namedIndex)
							observed := compileObservation(analysis.Diagnostics, false)
							if observed != native.Expected {
								for _, d := range analysis.Diagnostics {
									t.Logf("%s %s range=%+v native=%q", row.ID, d.Code, d.Range, d.NativeMessage)
								}
								t.Fatalf("%s named expected <%s> actual <%s>", row.ID, native.Expected, observed)
							}
							if observed == "compiled" {
								// The capture's compiler helper has no assertions.
								// Keep the row body and add the same exact row check
								// to its output helper for named execution.
								sources[native.Name] = strings.Replace(native.Source, "public void out(String id,Object v){}", "public void out(String id,Object v){String observedText=''+String.valueOf(v);System.assert('accepted'.equals(observedText),id+' expected <accepted> actual <'+observedText+'>');}", 1)
								invoke := compileNamed(t, build(sources), native.Name)
								invoke(t, "observed")
							}
							return
						}
						prepareNamed(t)
						namedInvoke(t, "observed"+row.ID)
					})
				})
			}
		})
	}
}
