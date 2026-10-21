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
// API 68 binds an anonymous capture (ceilingOracle in the fixture). It equals
// the API 67 oracle on 264 supported rows; its one missing row, R261, has a
// native API 68 uncaught recapture that equals the stored answer. The API 68
// @IsTest route runs locally against these anonymous answers and counts as
// legacy, not as observed native named behavior.
func TestSchemaProfilesOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected  string
		Compile, NativeNull bool
	}
	var data struct {
		APIVersions   []string `json:"apiVersions"`
		CeilingOracle struct {
			APIVersion, Route string
			OrgTSV            string   `json:"orgTSV"`
			OrgRowsSHA256     string   `json:"orgRowsSHA256"`
			Missing           []string `json:"missing"`
			UncaughtRecapture struct {
				Source, SHA256 string
				Rows           []string
			} `json:"uncaughtRecapture"`
		} `json:"ceilingOracle"`
		Declarations, Metadata map[string]string
		Prelude                string
		Cases                  []familyCase
		Remaining              []struct{ ID, Owner, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/schema_profiles.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 265 || len(data.Remaining) != 18 {
		t.Fatalf("oracle rows: %d supported + %d remaining", len(data.Cases), len(data.Remaining))
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" {
		t.Fatalf("oracle versions: %v", data.APIVersions)
	}
	ceiling := data.CeilingOracle
	if ceiling.APIVersion != "68.0" || ceiling.Route != "anonymous" || ceiling.OrgTSV == "" || len(ceiling.OrgRowsSHA256) != 64 {
		t.Fatalf("API 68 source binding: %#v", ceiling)
	}
	if strings.Join(ceiling.Missing, ",") != "R261" {
		t.Fatalf("API 68 org.tsv missing rows: %v, want R261", ceiling.Missing)
	}
	recapture := ceiling.UncaughtRecapture
	if recapture.Source == "" || len(recapture.SHA256) != 64 || strings.Join(recapture.Rows, ",") != "R261" {
		t.Fatalf("API 68 uncaught recapture: %#v", recapture)
	}
	recaptured68 := map[string]bool{}
	for _, id := range recapture.Rows {
		recaptured68[id] = true
	}
	uncaptured68 := map[string]bool{}
	for _, id := range ceiling.Missing {
		if !recaptured68[id] {
			uncaptured68[id] = true
		}
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
			cases := data.Cases
			if api == "68.0" {
				cases = nil
				for _, tc := range data.Cases {
					if uncaptured68[tc.ID] {
						t.Logf("%s excluded at API 68: no API 68 answer", tc.ID)
						continue
					}
					cases = append(cases, tc)
				}
			}
			if api == "68.0" {
				t.Log("API 68: anonymous capture plus native uncaught recapture of R261; @IsTest checks retain anonymous-backed local assertions without native named-route capture")
			}
			counts := newConformanceCounts()
			defer counts.Log(t, "schema profiles", api, "")
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
			namedSource.WriteString("@IsTest private class SchemaProfilesProbe {\n")
			accepted := 0
			for _, tc := range cases {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "SchemaProfilesProbe.cls")
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
					if result.ClassName != "SchemaProfilesProbe" || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if run.Summary().Total != accepted || len(namedResults) != accepted {
				t.Fatalf("named results: %#v, unique %d expected %d: %s", run.Summary(), len(namedResults), accepted, firstRunProblem(run))
			}
			for _, tc := range cases {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					kind := "exact"
					if rejected {
						kind = "category"
					}
					counts.RunKind(t, "anonymous", tc.ID, "anonymous", kind, func(t *testing.T) {
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
					namedKind := kind
					if api == "68.0" {
						namedKind = "legacy"
					}
					counts.RunKind(t, "isTest", tc.ID, "isTest", namedKind, func(t *testing.T) {
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
						path := filepath.Join(root, "SchemaProfilesRejected.cls")
						writeFile(t, path, "@IsTest private class SchemaProfilesRejected { @IsTest static void observed(){"+source+"} }")
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

// Reuse the captured numeric-text answer; the field-source variants exercise
// local propagation of that same String value rather than new native rows.
func TestSchemaProfileNumberPutFromStringField(t *testing.T) {
	var data struct {
		APIVersions []string `json:"apiVersions"`
		Metadata    map[string]string
		Cases       []struct{ ID, Code, Expected string }
	}
	raw, err := os.ReadFile("testdata/conformance/schema_profiles.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	var code, expected string
	for _, row := range data.Cases {
		if row.ID == "R259" {
			code, expected = row.Code, row.Expected
		}
	}
	if code == "" || expected != "12" || strings.Count(code, "'12'") != 1 {
		t.Fatal("missing numeric-text native answer")
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(root, path), source)
			}
			bodies := map[string]string{}
			var named strings.Builder
			named.WriteString("@IsTest private class NumberPutProbe {\n")
			for _, input := range []struct{ name, setup, expression string }{
				{"literal", "", "'12'"},
				{"textField", "Account sourceRecord=new Account(Name='12');", "sourceRecord.Name"},
				{"picklistField", "Account sourceRecord=new Account(Industry='12');", "sourceRecord.Industry"},
			} {
				body := input.setup + "Object r;" + strings.Replace(code, "'12'", input.expression, 1) +
					"Decimal amount=(Decimal)r;System.assertEquals(" + expected + ",amount);" + `
String rejected;
try { x.put('Amount__c','not-a-number'); }
catch(SObjectException e) { rejected=e.getMessage(); }
System.assertEquals('Illegal assignment from String to Decimal',rejected);
System.assertEquals(amount,x.Amount__c);`
				bodies[input.name] = body
				fmt.Fprintf(&named, "@IsTest static void %s(){%s}\n", input.name, body)
			}
			named.WriteString("}\n")
			path := filepath.Join(root, "force-app/main/default/classes/NumberPutProbe.cls")
			writeFile(t, path, named.String())
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if analysis := sema.Analyze(index); analysis.HasErrors() {
				t.Fatal(analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for name, body := range bodies {
				t.Run(name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != len(bodies) || summary.Passed != len(bodies) {
				t.Fatalf("named results: %#v: %s", summary, firstRunProblem(run))
			}
		})
	}
}
