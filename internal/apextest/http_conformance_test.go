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

// The HTTP oracle includes the owned REST handler and mock implementations.
// Both API endpoints agree; CI uses only these exported sources and values.
func TestHTTPOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected, Group string
		Compile                   bool
		Isolated                  bool
		NativeDiagnostic          string
		NamedOwner, NamedReason   string
		Owner, Reason             string
	}
	var data struct {
		APIVersions  []string `json:"apiVersions"`
		Declarations map[string]string
		Cases        []familyCase
		Controls     []familyCase
		Remaining    []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/http.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 298 || len(data.Controls) != 132 || len(data.Remaining) != 1 || strings.Join(data.APIVersions, ",") != "62.0,67.0" || len(data.Declarations) != 5 {
		t.Fatalf("oracle shape: rows=%d controls=%d carried=%d versions=%v helpers=%d", len(data.Cases), len(data.Controls), len(data.Remaining), data.APIVersions, len(data.Declarations))
	}
	data.Cases = append(data.Cases, data.Controls...)
	// SH001-SH022 preserve shadow capture S001-S022; the SH prefix keeps their
	// matrix identities distinct from the original SOAP controls S001-S015.
	seen := make(map[string]bool)
	for _, row := range data.Cases {
		if row.ID == "" || row.Code == "" || seen[row.ID] {
			t.Fatalf("invalid oracle row: %#v", row)
		}
		seen[row.ID] = true
	}
	for _, row := range data.Remaining {
		if row.ID == "" || seen[row.ID] || row.Owner == "" || row.Reason == "" || strings.Contains(row.Owner, "A40") {
			t.Fatalf("invalid carried row: %#v", row)
		}
		seen[row.ID] = true
		t.Logf("%s carried to %s: %s; native <%s>", row.ID, row.Owner, row.Reason, row.Expected)
	}
	// Escape physical line breaks exactly as the capture transport does. A
	// native-null row checks the raw value before any String conversion.
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=''+String.valueOf(v);
  observedText=observedText.replace('\r','\\r').replace('\n','\\n');
  if(expectedNull){
   System.assert(v==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText=('EXC|'+e.getTypeName()+'|'+e.getMessage()).replace('\r','\\r').replace('\n','\\n');
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase, code string) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.Expected == "null" {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + code
		}
		return prefix + "try {Object r; " + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
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
			t.Fatal("native compilation rejected; Glade accepted")
		}
		if tc.NativeDiagnostic == "" {
			t.Fatal("missing native diagnostic")
		}
		// org.tsv records the first native diagnostic, so compare the first
		// Glade diagnostic as exact text rather than concatenating later errors.
		observed := messages[0]
		if observed != tc.NativeDiagnostic {
			t.Fatalf("expected <%s> actual <%s>", tc.NativeDiagnostic, observed)
		}
	}
	// Preserve the original row text while lifting anonymous declarations into
	// companion classes. Their method bodies and the remaining row are unchanged.
	declarations := func(code string) (map[string]string, string, bool) {
		parsed := apexast.NewParser().ParseSource("HTTPAnonymous.cls", code)
		out := make(map[string]string)
		masked := []byte(code)
		for _, decl := range parsed.Declarations {
			if decl.Kind != apexast.DeclarationClass {
				continue
			}
			start, end := decl.Range.Start.Offset, decl.Range.End.Offset
			if start < 0 || end <= start || end > len(code) {
				return nil, code, true
			}
			out[decl.Name] = "public " + code[start:end]
			for i := start; i < end; i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
		}
		return out, string(masked), len(out) > 0 && (apexast.Result{Diagnostics: parsed.Diagnostics}).HasErrors()
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "HTTP callouts", api)
			root := t.TempDir()
			writeSources := func(root string, sources map[string]string) []string {
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
				return paths
			}
			sources := make(map[string]string)
			for name, source := range data.Declarations {
				sources[name] = source
			}
			sources["P"] = helper
			basePaths := writeSources(root, sources)
			build := func(extra []string) typesys.Index {
				files := append(append([]string{}, basePaths...), extra...)
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, gladeschema.Schema{})
			}
			// Source-shadow controls isolate their declarations from the original
			// mock helpers, whose unqualified HTTP types belong to the base oracle.
			buildIsolated := func(extra []string) typesys.Index {
				files := append([]string{filepath.Join(root, "P.cls")}, extra...)
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, gladeschema.Schema{})
			}
			base := build(nil)
			if base.HasErrors() || sema.Analyze(base).HasErrors() {
				t.Fatalf("helpers: parser=%v semantics=%v", base.Diagnostics, sema.Analyze(base).Diagnostics)
			}
			org := orgFromIndex(base)
			runner := newConformanceRunner(t, base, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class HTTPProbe {\n")
			grouped := make(map[string]bool)
			for _, tc := range data.Cases {
				decls, _, invalid := declarations(tc.Code)
				if invalid || len(decls) > 0 || tc.NamedOwner != "" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				grouped[tc.ID] = true
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, tc.Code))
			}
			namedSource.WriteString("}\n")
			// Compile each named matrix once, execute only selected subtests, and
			// clone the shared API base through the ordinary test-case executor.
			prepareNamed := func(t *testing.T, index typesys.Index) func(*testing.T, string) {
				t.Helper()
				if index.HasErrors() || sema.Analyze(index).HasErrors() {
					t.Fatalf("named compilation: parser=%v semantics=%v", index.Diagnostics, sema.Analyze(index).Diagnostics)
				}
				namedRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
				cases := Discover(index, Options{SelectedClasses: []string{"HTTPProbe"}})
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				counters := newRunPerfCounters(false)
				return func(t *testing.T, id string) {
					t.Helper()
					for _, tc := range cases {
						if tc.MethodName != "observed"+id {
							continue
						}
						key := testCaseKey(tc)
						seed := namedRunner.org.CloneRuntimeOrg()
						initializeTestOrg(&seed)
						result := runCase(context.Background(), tc, methods[key], runtimeMethods[testMethodSourceKey(tc.ClassName, tc.File)], methodErrors[key], programs[key], programErrors[key], namedRunner.base, nil, nil, nil, seed, 0, Options{NoDiskCache: true, Parallelism: 1}, false, nil, counters)
						if result.Status != testreport.StatusPass {
							t.Fatalf("named row: status=%s problem=%v", result.Status, result.Problem)
						}
						return
					}
					t.Fatalf("missing named method observed%s", id)
				}
			}
			var executeGrouped func(*testing.T, string)
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					decls, code, invalid := declarations(tc.Code)
					rowPaths := []string{}
					rowIndex := base
					rowBuild := build
					if tc.Isolated {
						rowBuild = buildIsolated
					}
					if len(decls) > 0 && !invalid {
						rowPaths = writeSources(t.TempDir(), decls)
						rowIndex = rowBuild(rowPaths)
					}
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					source := body(tc, code)
					counts.run(t, "anonymous", "anonymous", "exact", func(t *testing.T) {
						analysis := sema.AnalyzeAnonymous(rowIndex, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						diagnostics := append(append([]diagnostic.Diagnostic{}, rowIndex.Diagnostics...), analysis.Diagnostics...)
						if len(decls) > 0 {
							diagnostics = append(diagnostics, sema.Analyze(rowIndex).Diagnostics...)
						}
						if rejected {
							assertRejection(t, tc, diagnostics, compileErr)
							return
						}
						if invalid || rowIndex.HasErrors() || analysis.HasErrors() || compileErr != nil || (len(decls) > 0 && sema.Analyze(rowIndex).HasErrors()) {
							t.Fatalf("anonymous compile: declarations=%t error=%v diagnostics=%v", invalid, compileErr, diagnostics)
						}
						rowRunner := runner
						if len(decls) > 0 {
							rowRunner = newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
						}
						if _, err := rowRunner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					namedKind := "exact"
					if tc.NamedOwner != "" {
						namedKind = ""
					}
					counts.run(t, "isTest", "@IsTest", namedKind, func(t *testing.T) {
						if tc.NamedOwner != "" {
							t.Logf("%s @IsTest carried to %s: %s; anonymous native <%s>", tc.ID, tc.NamedOwner, tc.NamedReason, tc.Expected)
							return
						}
						if grouped[tc.ID] {
							if executeGrouped == nil {
								paths := writeSources(root, map[string]string{"HTTPProbe": namedSource.String()})
								executeGrouped = prepareNamed(t, build(paths))
							}
							executeGrouped(t, tc.ID)
							return
						}
						path := writeSources(t.TempDir(), map[string]string{"HTTPProbe": "@IsTest private class HTTPProbe { @IsTest static void observed" + tc.ID + "(){\n" + source + "\n} }"})
						namedIndex := rowBuild(append(append([]string{}, rowPaths...), path...))
						if !rejected {
							prepareNamed(t, namedIndex)(t, tc.ID)
							return
						}
						analysis := sema.Analyze(namedIndex)
						diagnostics := append(append([]diagnostic.Diagnostic{}, namedIndex.Diagnostics...), analysis.Diagnostics...)
						var compileErr error
						if !invalid && !namedIndex.HasErrors() && !analysis.HasErrors() {
							cases := Discover(namedIndex, Options{SelectedClasses: []string{"HTTPProbe"}})
							_, methodErrors := compileTestMethods(cases)
							for _, err := range methodErrors {
								if err != nil {
									compileErr = err
									break
								}
							}
						}
						assertRejection(t, tc, diagnostics, compileErr)
					})
				})
			}
		})
	}
}
