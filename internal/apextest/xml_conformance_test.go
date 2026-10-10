package apextest

import (
	"encoding/json"
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

// Owned Salesforce observations at both API endpoints. CI needs no org or tools.
func TestXMLOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected string
		Compile, Uncaught  bool
		RemainingReason    string `json:"remainingReason"`
	}
	var data struct {
		APIVersions                []string `json:"apiVersions"`
		Declarations               map[string]string
		Cases, Controls, Remaining []familyCase
	}
	raw, err := os.ReadFile("testdata/conformance/xml.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 278 || len(data.Controls) != 30 || len(data.Remaining) != 1 {
		t.Fatalf("oracle rows: %d main + %d controls + %d remaining", len(data.Cases), len(data.Controls), len(data.Remaining))
	}
	for _, row := range data.Remaining {
		if row.RemainingReason == "" {
			t.Fatalf("unowned remaining row %s", row.ID)
		}
		t.Logf("remaining %s: %s", row.ID, row.RemainingReason)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	// Inline capture helpers need public visibility when written as .cls files.
	helper := `public class P {
 public String expectedText;
 public Boolean nativeNull;
 public void out(String id,Object v){
  if(nativeNull){System.assert(v == null,id + ' expected raw null');return;}
  String observedText=String.valueOf(v);
  System.assert(expectedText.equals(observedText), id + ' expected <' + expectedText + '> actual <' + observedText + '>');
 }
}`
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + "; pq.nativeNull=" + fmt.Sprint(tc.Expected == "null") + ";\n"
		if tc.Compile {
			return prefix + tc.Code
		}
		code := tc.Code
		if !strings.Contains(code, ";") {
			code = "r=" + code + ";"
		}
		return prefix + "Object r; " + code + " pq.out('" + tc.ID + "',r);"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "XML", api)
			root := t.TempDir()
			names := make([]string, 0, len(data.Declarations)+1)
			declarations := make(map[string]string, len(data.Declarations)+1)
			for name, source := range data.Declarations {
				declarations[name] = source
				names = append(names, name)
			}
			declarations["P"] = helper
			names = append(names, "P")
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
				return typesys.Build(project.Project{Root: root, ApexFiles: files}, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if a := sema.Analyze(index); a.HasErrors() {
				t.Fatalf("helper semantics: %v", a.Diagnostics)
			}
			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true})
			// All accepted rows share one named-class compilation, but each @IsTest
			// method gets its own runner state. Rejected rows retain isolated compilation.
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class XMLProbe {\n")
			accepted := 0
			for _, tc := range rows {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "XMLProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if a := sema.Analyze(namedIndex); a.HasErrors() {
				t.Fatalf("named semantics: %v", a.Diagnostics)
			}
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := map[string]testreport.Case{}
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "XMLProbe" || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if run.Summary().Total != accepted || len(namedResults) != accepted {
				t.Fatalf("named results: %#v, unique%d expected%d: %s", run.Summary(), len(namedResults), accepted, firstRunProblem(run))
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if rejected {
						kind = "category"
					}
					counts.run(t, "anonymous", "anonymous", kind, func(t *testing.T) {
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
						_, err := anonymousRunner.execute(program)
						if tc.Uncaught {
							if err == nil || "UNCAUGHT|"+err.Error() != tc.Expected {
								t.Fatalf("uncaught oracle %q, got %v", tc.Expected, err)
							}
						} else if err != nil {
							t.Fatal(err)
						}
					})
					counts.run(t, "isTest", "@IsTest", kind, func(t *testing.T) {
						if !rejected {
							result := namedResults[tc.ID]
							if tc.Uncaught {
								if result.Status == testreport.StatusPass || result.Problem == nil || "UNCAUGHT|"+result.Problem.Message != tc.Expected {
									t.Fatalf("uncaught named oracle %q, got %#v", tc.Expected, result)
								}
								return
							}
							if result.Status != testreport.StatusPass {
								t.Fatalf("named row: status%s problem%v", result.Status, result.Problem)
							}
							return
						}
						path := filepath.Join(root, "XMLRejected.cls")
						writeFile(t, path, "@IsTest private class XMLRejected { @IsTest static void observed(){"+source+"} }")
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
