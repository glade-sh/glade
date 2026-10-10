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

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Salesforce captures at API 62/67 supply every expected row. The exported
// inbound implementation, folder and templates keep both CI routes independent
// of the capture project and Salesforce credentials.
func TestMessagingOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected, Group string
		Compile                   bool
		NativeDiagnostic          string
		FloorExpected             string
	}
	var data struct {
		APIVersions            []string `json:"apiVersions"`
		Declarations           map[string]string
		DeclarationAPIVersions map[string]string
		Metadata               map[string]string
		Cases                  []familyCase
		Controls               []familyCase
		CarriedControls        []struct{ ID, Code, Expected, Owner, Reason string }
		HostedControls         []struct{ ID, Expected, Reason string }
	}
	raw, err := os.ReadFile("testdata/conformance/messaging.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 298 || len(data.Controls) != 31 || len(data.Declarations) != 4 || len(data.DeclarationAPIVersions) != 4 || len(data.Metadata) != 7 || len(data.CarriedControls) != 1 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle shape: rows=%d controls=%d helpers=%d metadata=%d versions=%v", len(data.Cases), len(data.Controls), len(data.Declarations), len(data.Metadata), data.APIVersions)
	}
	rows := append(data.Cases, data.Controls...)
	seen := make(map[string]bool)
	for _, row := range rows {
		if row.ID == "" || row.Code == "" || seen[row.ID] {
			t.Fatalf("invalid/repeated row: %#v", row)
		}
		seen[row.ID] = true
	}
	for _, row := range data.HostedControls {
		t.Logf("%s excluded hosted-delivery control: %s; native <%s>", row.ID, row.Reason, row.Expected)
	}
	for _, row := range data.CarriedControls {
		t.Logf("%s carried: %s; owner %s; native <%s>", row.ID, row.Reason, row.Owner, row.Expected)
	}
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
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\nA41 a41=new A41();\n"
		if tc.Expected == "null" {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	assertRejection := func(t *testing.T, tc familyCase, diagnostics []diagnostic.Diagnostic, compileErr error) {
		t.Helper()
		for _, d := range diagnostics {
			if d.Severity != diagnostic.Error {
				continue
			}
			if d.Message != tc.NativeDiagnostic {
				t.Fatalf("expected native diagnostic <%s> actual <%s>", tc.NativeDiagnostic, d.Message)
			}
			return
		}
		if compileErr == nil {
			t.Fatal("native compilation rejected; Glade accepted")
		}
		if compileErr.Error() != tc.NativeDiagnostic {
			t.Fatalf("expected native diagnostic <%s> actual <%s>", tc.NativeDiagnostic, compileErr)
		}
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCountLog(t, "messaging", api)
			apiRows := append([]familyCase(nil), rows...)
			for i := range apiRows {
				if api == "62.0" && apiRows[i].FloorExpected != "" {
					apiRows[i].Expected = apiRows[i].FloorExpected
					_, apiRows[i].NativeDiagnostic, _ = strings.Cut(apiRows[i].Expected, ": ")
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(root, path), source)
			}
			writeSources := func(sources map[string]string) []string {
				names := make([]string, 0, len(sources))
				for name := range sources {
					names = append(names, name)
				}
				sort.Strings(names)
				paths := make([]string, 0, len(names))
				for _, name := range names {
					path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
					writeFile(t, path, sources[name])
					sourceAPI := api
					if capturedAPI := data.DeclarationAPIVersions[name]; capturedAPI != "" {
						sourceAPI = capturedAPI
					}
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+sourceAPI+"</apiVersion></ApexClass>")
					paths = append(paths, path)
				}
				return paths
			}
			sources := make(map[string]string, len(data.Declarations)+1)
			for name, source := range data.Declarations {
				sources[name] = source
			}
			sources["P"] = helper
			basePaths := writeSources(sources)
			build := func(extra []string) typesys.Index {
				files := append(append([]string{}, basePaths...), extra...)
				return typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, gladeschema.Schema{})
			}
			base := build(nil)
			if base.HasErrors() || sema.Analyze(base).HasErrors() {
				t.Fatalf("helpers: parser=%v semantics=%v", base.Diagnostics, sema.Analyze(base).Diagnostics)
			}
			org := orgFromIndex(base)
			runner := newConformanceRunner(t, base, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest private class MessagingProbe {\n")
			for _, tc := range apiRows {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR") {
					continue
				}
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			// One compilation and linked shared runner per source API. Each selected
			// row still executes through the normal @IsTest executor on a fresh org.
			var executeNamed func(*testing.T, string)
			prepareNamed := func() {
				paths := writeSources(map[string]string{"MessagingProbe": namedSource.String()})
				index := build(paths)
				analysis := sema.Analyze(index)
				if index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("named compilation: parser=%v semantics=%v", index.Diagnostics, analysis.Diagnostics)
				}
				namedRunner := newConformanceRunner(t, index, conformanceRunnerOptions{Base: runner, LinkProject: true, Org: &org})
				cases := Discover(index, Options{SelectedClasses: []string{"MessagingProbe"}})
				methods, methodErrors := compileTestMethods(cases)
				programs, programErrors := compileTestInvokePrograms(cases)
				runtimeMethods := indexTestRuntimeMethods(methods)
				counters := newRunPerfCounters(false)
				executeNamed = func(t *testing.T, id string) {
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
			for _, tc := range apiRows {
				t.Run(tc.ID, func(t *testing.T) {
					source := body(tc)
					rejected := strings.HasPrefix(tc.Expected, "COMPILE_ERROR")
					t.Run("anonymous", func(t *testing.T) {
						counts.Track(t, "anonymous", 1)
						analysis := sema.AnalyzeAnonymous(base, source, api)
						program, compileErr := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if rejected {
							assertRejection(t, tc, analysis.Diagnostics, compileErr)
							return
						}
						if analysis.HasErrors() || compileErr != nil {
							t.Fatalf("anonymous compile: error=%v diagnostics=%v", compileErr, analysis.Diagnostics)
						}
						if _, err := runner.execute(program); err != nil {
							t.Fatal(err)
						}
					})
					t.Run("isTest", func(t *testing.T) {
						counts.Track(t, "@IsTest", 1)
						if !rejected {
							if executeNamed == nil {
								prepareNamed()
							}
							executeNamed(t, tc.ID)
							return
						}
						paths := writeSources(map[string]string{"MessagingRejected": "@IsTest private class MessagingRejected { @IsTest static void observed(){\n" + source + "\n} }"})
						index := build(paths)
						analysis := sema.Analyze(index)
						diagnostics := append(append([]diagnostic.Diagnostic{}, index.Diagnostics...), analysis.Diagnostics...)
						var compileErr error
						if !index.HasErrors() && !analysis.HasErrors() {
							cases := Discover(index, Options{SelectedClasses: []string{"MessagingRejected"}})
							_, errors := compileTestMethods(cases)
							for _, err := range errors {
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
