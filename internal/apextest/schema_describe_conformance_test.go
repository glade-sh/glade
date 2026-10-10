package apextest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce observations at both source API endpoints; all inputs are local.
func TestSchemaDescribeOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
	}
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Declarations, Metadata map[string]string
		Prelude                string
		Cases                  []familyCase
		Remaining              []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/schema_describe.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 293 || len(data.Remaining) != 3 {
		t.Fatalf("oracle rows: %d supported + %d remaining", len(data.Cases), len(data.Remaining))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	seen := map[string]bool{}
	for _, tc := range data.Cases {
		if seen[tc.ID] || tc.ID == "" || tc.Expected == "?" {
			t.Fatalf("invalid/repeated row: %#v", tc)
		}
		seen[tc.ID] = true
	}
	for _, row := range data.Remaining {
		if seen[row.ID] || row.Owner == "" || row.Reason == "" {
			t.Fatalf("invalid remaining row: %#v", row)
		}
		seen[row.ID] = true
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
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n" + data.Prelude + "\n"
		if tc.NativeNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
			return prefix + "Object r; " + tc.Code + " System.assert(false,'expected uncatchable exception');"
		}
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "schema describe", api)
			root := t.TempDir()
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(root, path), source)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			declarations := map[string]string{"P": helper}
			for name, source := range data.Declarations {
				declarations[name] = source
			}
			var names, paths []string
			for name := range declarations {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				pp := p
				pp.ApexFiles = append([]string{}, paths...)
				if extra != "" {
					pp.ApexFiles = append(pp.ApexFiles, extra)
				}
				return typesys.Build(pp, schema)
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
			namedSource.WriteString("@IsTest private class SchemaDescribeProbe {\n")
			accepted := 0
			for _, tc := range data.Cases {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "SchemaDescribeProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if analysis := sema.Analyze(namedIndex); analysis.HasErrors() {
				t.Fatalf("named semantics: %v", analysis.Diagnostics)
			}
			// The class metadata selects the source API version for named execution.
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "SchemaDescribeProbe" || namedResults[id].MethodName != "" {
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
					source := body(tc)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if rejected {
						kind = "category"
					}
					t.Run("anonymous", func(t *testing.T) {
						counts.TrackKind(t, "anonymous", kind, 1)
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if rejected {
							if compileErr == nil && !analysis.HasErrors() {
								t.Fatal("org rejects compilation; anonymous path accepted")
							}
							return
						}
						if analysis.HasErrors() {
							t.Fatalf("anonymous semantics: %v", analysis.Diagnostics)
						}
						if compileErr != nil {
							t.Fatal(compileErr)
						}
						_, err := runner.execute(program)
						if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
							observed := ""
							if err != nil {
								observed = err.Error()
								var runtimeErr *vm.RuntimeError
								if errors.As(err, &runtimeErr) {
									observed = runtimeErr.Message
								}
							}
							if want := strings.TrimPrefix(tc.Expected, "UNCAUGHT|"); observed != want {
								t.Fatalf("expected <%s> actual <%s>", want, observed)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						counts.TrackKind(t, "@IsTest", kind, 1)
						if !rejected {
							result := namedResults[tc.ID]
							if strings.HasPrefix(tc.Expected, "UNCAUGHT|") {
								want := strings.TrimPrefix(tc.Expected, "UNCAUGHT|")
								if result.Status != testreport.StatusFail || result.Problem == nil || result.Problem.Message != want {
									t.Fatalf("expected <%s>, named result: %#v problem: %#v", want, result, result.Problem)
								}
								return
							}
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
							}
							return
						}
						path := filepath.Join(root, "SchemaDescribeRejected.cls")
						writeFile(t, path, "@IsTest private class SchemaDescribeRejected { @IsTest static void observed(){"+source+"} }")
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						rejectedIndex := buildIndex(path)
						analysis := sema.Analyze(rejectedIndex)
						if !rejectedIndex.HasErrors() && !analysis.HasErrors() {
							r := Run(rejectedIndex, Options{NoDiskCache: true, Parallelism: 1})
							if r.Summary().CompileErrors == 0 {
								t.Fatalf("org rejects compilation; named path accepted: %#v %s", r.Summary(), firstRunProblem(r))
							}
						}
					})
				})
			}
		})
	}
}
