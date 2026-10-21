package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned source and native observations are sufficient for credential-free CI.
func TestSourceSyntaxOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected   string
		Compile              bool
		NamedExpected        string            `json:"namedExpected"`
		AnonymousOnly        bool              `json:"anonymousOnly"`
		NamedCompilationOnly bool              `json:"namedCompilationOnly"`
		NativeAnonymous      map[string]string `json:"nativeAnonymous"`
	}
	var data struct {
		APIVersions []string     `json:"apiVersions"`
		Cases       []familyCase `json:"cases"`
		Controls    []familyCase `json:"controls"`
	}
	raw, err := os.ReadFile("testdata/conformance/source_syntax.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 225 || len(data.Controls) != 30 || strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatalf("oracle shape: cases=%d controls=%d versions=%v", len(data.Cases), len(data.Controls), data.APIVersions)
	}
	data.Cases = append(data.Cases, data.Controls...)
	seen := map[string]bool{}
	for _, row := range data.Cases {
		if seen[row.ID] || row.Expected == "" {
			t.Fatalf("duplicate or missing oracle row: %#v", row)
		}
		seen[row.ID] = true
	}
	// The boundary test also checks exact C071/C072 diagnostics. These three
	// former carries are included here; their named observations are compile-only.
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
	body := func(tc familyCase, code, expected string) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(expected) + ";\n"
		if expected == "null" {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + code
		}
		return prefix + "try {Object r; " + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	assertRejection := func(t *testing.T, expected string, rejected bool) {
		t.Helper()
		expectedText, firstLine, _ := strings.Cut(expected, "\t")
		observedText := "compiled"
		if rejected {
			observedText = "COMPILE_ERROR"
		}
		if expectedText != observedText {
			t.Fatalf("source category expected <%s> actual <%s>", expectedText, observedText)
		}
		// The capture and exported wrappers have different source offsets and
		// diagnostic wording. A01 carries that first-line text separately.
		t.Logf("A01 compile diagnostic text carried: native <%s>; exact rejection category matched", firstLine)
	}
	// Transient declarations are checked in anonymous context before either
	// execution route. Named test execution uses public companion class files,
	// just as other conformance helpers do, rather than method-local classes.
	declarations := func(code string) (map[string]string, string, bool) {
		parsed := apexast.NewParser().ParseSource("SourceSyntaxAnonymous.cls", code)
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
				return nil, code, true
			}
			out[decl.Name] = code[start:end]
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
			counts := newConformanceCountLog(t, "source syntax", api)
			if api == "68.0" {
				t.Log("API68 V/G controls and named runtime checks are local regressions without native API68 route captures; C043 is anonymous-only")
			}
			build := func(root string, sources map[string]string) typesys.Index {
				t.Helper()
				paths := make([]string, 0, len(sources))
				for name, source := range sources {
					path := filepath.Join(root, name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, gladeschema.Schema{})
			}
			root := t.TempDir()
			base := build(root, map[string]string{"P": helper})
			if base.HasErrors() || sema.Analyze(base).HasErrors() {
				t.Fatalf("helper compile: %v %v", base.Diagnostics, sema.Analyze(base).Diagnostics)
			}
			org := orgFromIndex(base)
			runner := newConformanceRunner(t, base, conformanceRunnerOptions{LinkProject: true, Org: &org})
			namedExpected := func(tc familyCase) string {
				if tc.NamedExpected != "" {
					return tc.NamedExpected
				}
				return tc.Expected
			}
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class SourceSyntaxProbe {\n")
			grouped := map[string]bool{}
			for _, tc := range data.Cases {
				if tc.AnonymousOnly || tc.NamedCompilationOnly {
					continue
				}
				decls, _, invalid := declarations(tc.Code)
				if invalid || len(decls) > 0 || strings.HasPrefix(namedExpected(tc), "COMPILE_ERROR") {
					continue
				}
				grouped[tc.ID] = true
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc, tc.Code, namedExpected(tc)))
			}
			namedSource.WriteString("}\n")
			var namedResults map[string]testreport.Case
			prepareNamedResults := func(t *testing.T) {
				t.Helper()
				if namedResults != nil {
					return
				}
				// Rejected-row retries never need the accepted-row batch.
				namedIndex := build(root, map[string]string{"P": helper, "SourceSyntaxProbe": namedSource.String()})
				if namedIndex.HasErrors() || sema.Analyze(namedIndex).HasErrors() {
					t.Fatalf("named batch compile: %v %v", namedIndex.Diagnostics, sema.Analyze(namedIndex).Diagnostics)
				}
				namedRun := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
				namedResults = map[string]testreport.Case{}
				for _, suite := range namedRun.Suites {
					for _, result := range suite.Cases {
						id := strings.TrimPrefix(result.MethodName, "observed")
						if !grouped[id] || namedResults[id].MethodName != "" {
							t.Fatalf("unexpected or repeated named row: %#v", result)
						}
						namedResults[id] = result
					}
				}
				if namedRun.Summary().Total != len(grouped) || len(namedResults) != len(grouped) {
					t.Fatalf("named results: %#v unique=%d expected=%d: %s", namedRun.Summary(), len(namedResults), len(grouped), firstRunProblem(namedRun))
				}
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					sourceCode, exactSource := tc.NativeAnonymous[api]
					if !exactSource {
						sourceCode = tc.Code
					}
					decls, code, parseRejected := declarations(sourceCode)
					rowIndex := base
					if len(decls) > 0 && !parseRejected {
						if _, capturedHelper := decls["P"]; !capturedHelper {
							decls["P"] = helper
						}
						rowIndex = build(t.TempDir(), decls)
					}
					declarationRejected := parseRejected || rowIndex.HasErrors()
					if len(decls) > 0 && !declarationRejected {
						declarationRejected = sema.AnalyzeAnonymousDeclarations(rowIndex).HasErrors()
					}
					t.Run("anonymous", func(t *testing.T) {
						kind := "exact"
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							kind = "category"
						}
						if api == "68.0" && (strings.HasPrefix(tc.ID, "V") || strings.HasPrefix(tc.ID, "G")) {
							kind = "legacy"
						}
						counts.TrackKind(t, "anonymous", kind, 1)
						source := body(tc, code, tc.Expected)
						if exactSource {
							source = code
						}
						analysis := sema.AnalyzeAnonymous(rowIndex, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						rejected := declarationRejected || analysis.HasErrors() || compileErr != nil
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							assertRejection(t, tc.Expected, rejected)
							return
						}
						if rejected {
							t.Fatalf("anonymous compile: declarations=%t parse=%v semantics=%v", declarationRejected, compileErr, analysis.Diagnostics)
						}
						rowRunner := runner
						if exactSource {
							rowOrg := orgFromIndex(rowIndex)
							rowRunner = newConformanceRunner(t, rowIndex, conformanceRunnerOptions{LinkProject: true, Org: &rowOrg})
						} else if len(decls) > 0 {
							runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(rowIndex, nil)
							if err != nil {
								t.Fatal(err)
							}
							rowOrg := orgFromIndex(rowIndex)
							rowRunner = newConformanceRunner(t, rowIndex, conformanceRunnerOptions{Base: runner, RequestRuntime: &runtime, Org: &rowOrg})
						}
						_, runErr := rowRunner.execute(program)
						if want, uncaught := strings.CutPrefix(tc.Expected, "UNCAUGHT|"); uncaught {
							observed := ""
							if runErr != nil {
								observed = runErr.Error()
								var runtimeErr *vm.RuntimeError
								if errors.As(runErr, &runtimeErr) {
									observed = runtimeErr.ExceptionMessage()
									if runtimeErr.Type != "" && !strings.HasPrefix(observed, runtimeErr.Type+": ") {
										observed = runtimeErr.Type + ": " + observed
									}
								}
							}
							if observed != want {
								t.Fatalf("native uncaught <%s>; Glade <%s>", want, observed)
							}
							return
						}
						if runErr != nil {
							t.Fatal(runErr)
						}
					})
					if tc.AnonymousOnly {
						return
					}
					namedRoute := "isTest"
					if tc.NamedCompilationOnly {
						namedRoute = "namedCompilation"
					}
					t.Run(namedRoute, func(t *testing.T) {
						kind := "exact"
						if strings.HasPrefix(namedExpected(tc), "COMPILE_ERROR") {
							kind = "category"
						}
						if api == "68.0" && !tc.NamedCompilationOnly {
							kind = "legacy"
						}
						countRoute := "@IsTest"
						if tc.NamedCompilationOnly {
							countRoute = "named-compile"
						}
						counts.TrackKind(t, countRoute, kind, 1)
						if grouped[tc.ID] {
							prepareNamedResults(t)
							result := namedResults[tc.ID]
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status=%s problem=%v", result.Status, result.Problem)
							}
							return
						}
						if declarationRejected {
							if !strings.HasPrefix(namedExpected(tc), "COMPILE_ERROR") {
								t.Fatal("native accepts declarations; source declaration check rejected them")
							}
							assertRejection(t, namedExpected(tc), true)
							return
						}
						sources := map[string]string{"P": helper}
						namedDecls, namedCode, _ := declarations(tc.Code)
						for name, declaration := range namedDecls {
							if name == "P" {
								continue
							}
							// Anonymous types need explicit top-level visibility as files.
							if !strings.HasPrefix(strings.TrimSpace(declaration), "public ") && !strings.HasPrefix(strings.TrimSpace(declaration), "global ") {
								declaration = "public " + declaration
							}
							sources[name] = declaration
						}
						sources["SourceSyntaxIsolated"] = "@IsTest private class SourceSyntaxIsolated { @IsTest static void observed(){\n" + body(tc, namedCode, namedExpected(tc)) + "\n} }"
						isolatedIndex := build(t.TempDir(), sources)
						analysis := sema.Analyze(isolatedIndex)
						if strings.HasPrefix(namedExpected(tc), "COMPILE_ERROR") {
							assertRejection(t, namedExpected(tc), isolatedIndex.HasErrors() || analysis.HasErrors())
							return
						}
						if isolatedIndex.HasErrors() || analysis.HasErrors() {
							t.Fatalf("named isolated compile: %v %v", isolatedIndex.Diagnostics, analysis.Diagnostics)
						}
						if tc.NamedCompilationOnly {
							return
						}
						run := Run(isolatedIndex, Options{NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || run.Summary().Passed != 1 {
							t.Fatalf("named isolated: %#v %s", run.Summary(), firstRunProblem(run))
						}
					})
				})
			}
		})
	}
}
