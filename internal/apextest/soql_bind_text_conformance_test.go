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

// Owned SOQL selection observations at APIs 62, 65 and 67. CI uses local inputs.
func TestSOQLBindTextOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
		CompileOnly         bool
		RuntimeCompile      bool
		NativeLine          int
	}
	var data struct {
		APIVersions    []string `json:"apiVersions"`
		Declarations   map[string]string
		Prelude        string
		NamedSources   map[string]map[string]string
		IsTestSources  map[string]string
		IsTestExpected map[string]map[string]string
		Cases          []familyCase
		Remaining      []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/soql_bind_text.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 50 || len(data.Remaining) != 0 {
		t.Fatalf("oracle rows: %d exact-text cases + %d unasserted", len(data.Cases), len(data.Remaining))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,65.0,67.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	seen := map[string]bool{}
	for _, tc := range data.Cases {
		if seen[tc.ID] || tc.ID == "" || tc.Expected == "?" || strings.HasPrefix(tc.Expected, "COMPILE_ERROR") && !tc.Compile {
			t.Fatalf("invalid/repeated asserted row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	for _, row := range data.Remaining {
		if seen[row.ID] || row.ID == "" || row.Owner == "" || row.Reason == "" {
			t.Fatalf("invalid/repeated unasserted row: %#v", row)
		}
		seen[row.ID] = true
		if row.Owner == "SOQL selection" {
			t.Fatalf("owned row cannot be carried: %#v", row)
		}
		t.Logf("carried %s owner=%s: %s", row.ID, row.Owner, row.Reason)
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
  if(expectedText=='null'){System.assert(v!=null,id+' expected String null, actual raw null');}
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase) string {
		if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
			return tc.Code
		}
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		return prefix + data.Prelude + "\ntry {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	rejectedSource := func(tc familyCase) string {
		prefix := "P pq=new P(); \n"
		line := strings.Count(prefix, "\n") + 1
		if tc.NativeLine < line {
			t.Fatalf("native line %d is before rejection source for %s", tc.NativeLine, tc.ID)
		}
		predicateLine := strings.Count(tc.Code[:strings.Index(tc.Code, "List<Account> rows=")], "\n")
		prefix += strings.Repeat("\n", tc.NativeLine-line-predicateLine)
		code := tc.Code
		if tc.RuntimeCompile {
			code = "try {Object r; " + code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
		}
		prefix += code
		return prefix
	}
	compileText := func(result sema.Result, tc familyCase) string {
		for _, item := range result.Diagnostics {
			if item.Severity != diagnostic.Error {
				continue
			}
			line := 0
			if item.Range != nil {
				line = item.Range.Start.Line
			}
			observed := strings.ReplaceAll(item.Message, "\n", " ")
			if tc.NativeLine > 0 {
				observed = fmt.Sprintf("line %d: %s", line, observed)
			}
			// probe.py preserves native compile payloads up to 300 characters.
			if len(observed) > 300 {
				observed = observed[:300]
			}
			return "COMPILE_ERROR\t" + observed
		}
		return "compiled"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			// C002 rejects against known standard schema. An empty schema lets
			// source inference invent fields; one captured Id field keeps it authoritative,
			// with its remaining standard fields supplied by the shared catalog.
			oracleSchema := gladeschema.Schema{}
			for _, name := range []string{"Account"} {
				oracleSchema.Objects = append(oracleSchema.Objects, gladeschema.Object{
					Name: name, Fields: []gladeschema.Field{{Name: "Id", Type: "Id"}},
				})
			}
			names := make([]string, 0, len(data.Declarations)+1)
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
			}
			for name := range declarations {
				names = append(names, name)
			}
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
				return typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, oracleSchema)
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
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			// Native named execution has its own oracle and exact captured source.
			// Replace only the deliberate terminal observation assertions with
			// strict equality; keep query bodies and their lexical scopes intact.
			if len(data.IsTestExpected[api]) != 48 {
				t.Fatalf("named oracle rows: %d, expected 48", len(data.IsTestExpected[api]))
			}
			namedSource := data.IsTestSources[api]
			executable := 0
			for _, tc := range data.Cases {
				if tc.CompileOnly {
					continue
				}
				expected, observed := data.IsTestExpected[api][tc.ID]
				if !observed || expected == "?" {
					t.Fatalf("missing native named observation for %s", tc.ID)
				}
				if strings.HasPrefix(expected, "COMPILE_ERROR") {
					continue
				}
				executable++
				terminal := "System.assert(false,'WIDGET_BIND_ROWS|" + tc.ID + "|'+String.valueOf(r)+'|WIDGET_BIND_END');"
				if strings.Count(namedSource, terminal) != 1 {
					t.Fatalf("missing/repeated captured observation assertion for %s", tc.ID)
				}
				assertion := "System.assert(" + conformanceApexString(expected) + ".equals(String.valueOf(r)), '" + tc.ID + " actual <'+String.valueOf(r)+'>');"
				namedSource = strings.Replace(namedSource, terminal, assertion, 1)
			}
			namedClass := "WidgetBindMatrix" + strings.Split(api, ".")[0]
			namedPath := filepath.Join(root, namedClass+".cls")
			writeFile(t, namedPath, namedSource)
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			// Use the shared @IsTest runner for isolated method transactions,
			// source API selection and test execution context.
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != namedClass || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if executable != 45 || run.Summary().Total != executable || len(namedResults) != executable {
				t.Fatalf("named results: %#v, unique %d expected %d: %s", run.Summary(), len(namedResults), executable, firstRunProblem(run))
			}
			for _, tc := range data.Cases {
				t.Run(tc.ID, func(t *testing.T) {
					if tc.CompileOnly {
						t.Run("namedCompiler", func(t *testing.T) {
							source := data.NamedSources[api][tc.ID]
							if source == "" {
								t.Fatal("missing captured named source")
							}
							path := filepath.Join(root, "WidgetNative"+strings.Split(api, ".")[0]+tc.ID+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							analysis := sema.Analyze(buildIndex(path))
							if observed := compileText(analysis, tc); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
						})
						return
					}

					t.Run("anonymous", func(t *testing.T) {
						source := body(tc)
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							source = rejectedSource(tc)
						}
						analysis := sema.AnalyzeAnonymous(index, source, api)
						if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
							if observed := compileText(analysis, tc); observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, err = anonymousRunner.execute(program)
						if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
							observed := "UNCAUGHT|"
							if err != nil {
								observed += err.Error()
							}
							if observed != tc.Expected {
								t.Fatalf("expected <%s> actual <%s>", tc.Expected, observed)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						expected := data.IsTestExpected[api][tc.ID]
						if strings.HasPrefix(expected, "COMPILE_ERROR") {
							source := data.NamedSources[api][tc.ID]
							if source == "" {
								t.Fatal("missing captured named rejection source")
							}
							path := filepath.Join(root, "WidgetBindControl"+strings.Split(api, ".")[0]+tc.ID+".cls")
							writeFile(t, path, source)
							writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							// Tooling class compilation captured the message without
							// executeAnonymous's line prefix; retain both exact texts.
							namedCase := tc
							namedCase.NativeLine = 0
							if actual := compileText(sema.Analyze(buildIndex(path)), namedCase); actual != expected {
								t.Fatalf("expected <%s> actual <%s>", expected, actual)
							}
							return
						}
						result := namedResults[tc.ID]
						if result.Status != testreport.StatusPass {
							t.Fatalf("named expected <%s>: status %s problem %v", expected, result.Status, result.Problem)
						}
					})
				})
			}
		})
	}
}
