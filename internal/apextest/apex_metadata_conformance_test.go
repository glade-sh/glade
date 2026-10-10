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

type apexMetadataOracleCase struct {
	ID, Group, Code, Expected, FloorExpected string
	Compile, NativeNull                      bool
	SourceLine                               int
	NamedOwner, NamedReason                  string
}

// Preserve the complete native diagnostic, including its captured source line.
func apexMetadataCompileText(diagnostics []diagnostic.Diagnostic) string {
	for _, item := range diagnostics {
		if item.Severity != diagnostic.Error {
			continue
		}
		message := item.Message
		if item.NativeMessage != "" {
			message = item.NativeMessage
		}
		line := 0
		if item.Range != nil {
			line = item.Range.Start.Line
		}
		if item.NativeLine != nil {
			line = *item.NativeLine
		}
		return fmt.Sprintf("COMPILE_ERROR\tline %d: %s", line, message)
	}
	return "compiled"
}

// The request compiler lifts these declarations out of anonymous bodies. Mask
// their exact ranges while keeping all statement bytes and line positions.
func apexMetadataDeclarations(t *testing.T, code string) (map[string]string, string) {
	t.Helper()
	parsed := apexast.NewParser().ParseSource("MetadataAnonymous.cls", code)
	declarations := map[string]string{}
	masked := []byte(code)
	for _, declaration := range parsed.Declarations {
		if declaration.Kind != apexast.DeclarationClass {
			continue
		}
		start, end := declaration.Range.Start.Offset, declaration.Range.End.Offset
		if start < 0 || end <= start || end > len(code) {
			t.Fatalf("invalid declaration range: %s", declaration.Name)
		}
		declarations[declaration.Name] = code[start:end]
		for i := start; i < end; i++ {
			if masked[i] != '\n' && masked[i] != '\r' {
				masked[i] = ' '
			}
		}
	}
	return declarations, string(masked)
}

// Owned native DTO, enum, clone and invalid-boundary observations. CI uses only
// the exported source/answers; it never calls Salesforce or imports probe tools.
func TestApexMetadataOrgConformance(t *testing.T) {
	var data struct {
		APIVersions     []string `json:"apiVersions"`
		Cases, Controls []apexMetadataOracleCase
		Declarations    map[string]string
	}
	raw, err := os.ReadFile("testdata/conformance/apex_metadata.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 262 || len(data.Controls) != 36 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle shape: cases=%d controls=%d APIs=%v", len(data.Cases), len(data.Controls), data.APIVersions)
	}
	rows := append(append([]apexMetadataOracleCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, row := range rows {
		if row.ID == "" || row.Code == "" || row.Expected != row.FloorExpected || seen[row.ID] {
			t.Fatalf("invalid oracle row: %#v", row)
		}
		// Empty strings are native values in R098/R148, not missing answers.
		if row.Expected == "" && row.ID != "R098" && row.ID != "R148" {
			t.Fatalf("missing oracle answer: %#v", row)
		}
		if row.NativeNull && row.Expected != "null" {
			t.Fatalf("invalid raw-null row: %#v", row)
		}
		if strings.HasPrefix(row.Expected, "COMPILE_ERROR") && row.SourceLine < 3 {
			t.Fatalf("rejection has no captured line: %#v", row)
		}
		if row.NamedOwner != "" && (row.NamedOwner == "A44" || row.NamedReason == "") {
			t.Fatalf("invalid named carry: %#v", row)
		}
		seen[row.ID] = true
	}
	checkCompileText := func(t *testing.T, row apexMetadataOracleCase, observed string) {
		t.Helper()
		if observed == row.Expected {
			return
		}
		t.Fatalf("expected <%s> actual <%s>", row.Expected, observed)
	}
	helper := `public class ApexMetadataOracleOutput {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object value){
  String observedText=''+String.valueOf(value);
  if(expectedNull){
   System.assert(value==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception error){
  String observedText='EXC|'+error.getTypeName()+'|'+error.getMessage();
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	codeFor := func(row apexMetadataOracleCase) string {
		_, code := apexMetadataDeclarations(t, row.Code)
		return code
	}
	body := func(row apexMetadataOracleCase) string {
		prefix := "ApexMetadataOracleOutput pq=new ApexMetadataOracleOutput(); pq.expectedText=" + conformanceApexString(row.Expected) + ";\n"
		if row.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		code := codeFor(row)
		if row.Compile {
			return prefix + code
		}
		return prefix + "try {Object r; " + code + " pq.out('" + row.ID + "',r);}catch(Exception e){pq.err('" + row.ID + "',e);}"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "Apex metadata", api)
			root := t.TempDir()
			baseSources := map[string]string{"ApexMetadataOracleOutput": helper}
			for name, source := range data.Declarations {
				baseSources[name] = source
			}
			build := func(directory string, sources map[string]string) typesys.Index {
				names := make([]string, 0, len(sources))
				for name := range sources {
					names = append(names, name)
				}
				sort.Strings(names)
				paths := make([]string, 0, len(names))
				for _, name := range names {
					path := filepath.Join(directory, name+".cls")
					writeFile(t, path, sources[name])
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				return typesys.Build(project.Project{Root: directory, ApexFiles: paths}, gladeschema.Schema{})
			}
			copySources := func() map[string]string {
				out := make(map[string]string, len(baseSources)+2)
				for name, source := range baseSources {
					out[name] = source
				}
				return out
			}
			index := build(root, baseSources)
			if index.HasErrors() || sema.Analyze(index).HasErrors() {
				t.Fatalf("helper compile: %v %v", index.Diagnostics, sema.Analyze(index).Diagnostics)
			}
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class ApexMetadataOracleTest {\n")
			namedCount := 0
			for _, row := range rows {
				if strings.HasPrefix(row.Expected, "COMPILE_ERROR") || row.NamedOwner != "" {
					continue
				}
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", row.ID, body(row))
				namedCount++
			}
			namedSource.WriteString("}\n")
			namedSources := copySources()
			namedSources["ApexMetadataOracleTest"] = namedSource.String()
			namedIndex := build(root, namedSources)
			if namedIndex.HasErrors() || sema.Analyze(namedIndex).HasErrors() {
				t.Fatalf("named compile: %v %v", namedIndex.Diagnostics, sema.Analyze(namedIndex).Diagnostics)
			}
			namedCases := Discover(namedIndex, Options{SelectedClasses: []string{"ApexMetadataOracleTest"}})
			if len(namedCases) != namedCount {
				t.Fatalf("named methods=%d want=%d", len(namedCases), namedCount)
			}
			methods, methodErrors := compileTestMethods(namedCases)
			programs, programErrors := compileTestInvokePrograms(namedCases)
			byID := map[string]TestCase{}
			for _, testCase := range namedCases {
				key := testCaseKey(testCase)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("named method %s: %v %v", key, methodErrors[key], programErrors[key])
				}
				byID[strings.TrimPrefix(testCase.MethodName, "observed")] = testCase
			}
			namedRunner := newConformanceRunner(t, namedIndex, conformanceRunnerOptions{Base: anonymousRunner, LinkProject: true})
			for _, row := range rows {
				t.Run(row.ID, func(t *testing.T) {
					if strings.HasPrefix(row.Expected, "COMPILE_ERROR") {
						decls, code := apexMetadataDeclarations(t, row.Code)
						prefix := "ApexMetadataOracleOutput pq=new ApexMetadataOracleOutput(); "
						if !row.Compile {
							prefix += "Object r; "
							code += " pq.out('" + row.ID + "',r);"
						}
						sources := copySources()
						for name, declaration := range decls {
							sources[name] = strings.Repeat("\n", row.SourceLine-1) + declaration
						}
						t.Run("anonymous", func(t *testing.T) {
							counts.Track(t, "anonymous", 1)
							rowIndex := index
							observed := "compiled"
							if len(decls) > 0 {
								rowIndex = sema.WithAnonymousDeclarationContext(build(t.TempDir(), sources))
								transient := rowIndex
								transient.Types = nil
								for _, typ := range rowIndex.Types {
									if _, own := decls[typ.Name]; own && !typ.Dependency {
										transient.Types = append(transient.Types, typ)
									}
								}
								observed = apexMetadataCompileText(sema.AnalyzeAnonymousDeclarationsInContext(rowIndex, transient).Diagnostics)
							}
							if observed == "compiled" {
								source := strings.Repeat("\n", row.SourceLine-1) + prefix + code
								observed = apexMetadataCompileText(sema.AnalyzeAnonymous(rowIndex, source, api).Diagnostics)
							}
							checkCompileText(t, row, observed)
						})
						t.Run("isTest", func(t *testing.T) {
							counts.Track(t, "@IsTest", 1)
							for name, declaration := range decls {
								sources[name] = strings.Repeat("\n", row.SourceLine-1) + "public " + declaration
							}
							// Two wrapper lines replace the capture prelude; all
							// errors retain their native row/declaration line.
							sources["ApexMetadataRejected"] = "@IsTest private class ApexMetadataRejected {\n@IsTest static void observed(){\n" + strings.Repeat("\n", row.SourceLine-3) + prefix + code + "\n}}"
							rejectedIndex := build(t.TempDir(), sources)
							observed := apexMetadataCompileText(rejectedIndex.Diagnostics)
							if observed == "compiled" {
								observed = apexMetadataCompileText(sema.Analyze(rejectedIndex).Diagnostics)
							}
							checkCompileText(t, row, observed)
						})
						return
					}
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						source := body(row)
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
					t.Run("isTest", func(t *testing.T) {
						if row.NamedOwner != "" {
							t.Logf("%s named route carried to %s: %s; anonymous native <%s>", row.ID, row.NamedOwner, row.NamedReason, row.Expected)
							return
						}
						counts.Track(t, "@IsTest", 1)
						testCase, exists := byID[row.ID]
						if !exists {
							t.Fatalf("missing named row %s", row.ID)
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
					})
				})
			}
		})
	}
}
