package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowSubflowScalarPrecedenceNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/flow_subflow_scalar_precedence"
	for _, api := range []string{"62.0", "67.0"} {
		for _, context := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+context, func(t *testing.T) {
				capture, err := os.ReadFile(filepath.Join(directory, "native", "api"+strings.TrimSuffix(api, ".0"), context, "RUN-FACTS.txt"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(capture), "native_complete=true\n") || !strings.Contains(string(capture), "api="+api+"\n") || !strings.Contains(string(capture), "context="+context+"\n") {
					t.Fatal("incomplete native capture")
				}
				root := t.TempDir()
				sourceRoot := filepath.Join(directory, "project")
				err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					source, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					source = []byte(strings.ReplaceAll(string(source), "62.0", api))
					rel, err := filepath.Rel(sourceRoot, path)
					if err != nil {
						return err
					}
					if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256(source))) {
						t.Fatalf("native source changed: %s", rel)
					}
					writeFile(t, filepath.Join(root, rel), string(source))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				raw, err := os.ReadFile(filepath.Join(directory, "native", "api"+strings.TrimSuffix(api, ".0"), context, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
				if len(rows) != 19 || rows[0] != "route\tobservation" {
					t.Fatal("incomplete native route matrix")
				}
				want := map[string]string{}
				var routes []string
				for _, row := range rows[1:] {
					key, value, ok := strings.Cut(row, "\t")
					if !strings.Contains(key, "/Record") || strings.Contains(key, "/invocable/") {
						continue
					}
					if !ok {
						t.Fatal("invalid native row")
					}
					if _, exists := want[key]; exists {
						t.Fatalf("duplicate native route: %s", key)
					}
					var observation string
					if err := json.Unmarshal([]byte(value), &observation); err != nil {
						t.Fatal(err)
					}
					want[key] = observation
					routes = append(routes, key)
				}
				// Replay the captured record-variable slice through both interview entries.
				// Null assignment to an automatic lookup resource has a separate loader
				// boundary; its native rows remain available and its binding is unit-tested.
				// Invocable actions discard explicit null record inputs before assignment;
				// the non-null invocable twin is covered by the record-boundary matrix.
				const replay = `
Account seed=new Account(Name='Binding seed');insert seed;
Map<String,String> output=new Map<String,String>();
for(String route:new List<String>{'dynamic','static'}) {
 output.put('direct/'+route+'/Record',BindingProbe.invoke(route,'Record',seed.Id));
 for(String operation:new List<String>{'insert','update'}) {
  Contact item=new Contact(LastName=operation=='insert' ? 'Binding '+route+' Record' : 'Unrelated',AccountId=seed.Id);
  insert item;
  if(operation=='update') {item.LastName='Binding '+route+' Record';update item;}
  output.put('trigger/'+route+'/Record/'+operation,[SELECT Description FROM Contact WHERE Id=:item.Id].Description);
 }
}
System.assert(false,'OBS|'+JSON.serialize(output));`
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/BindingReplayTest.cls"), "@IsTest private class BindingReplayTest { @IsTest static void observeScalarPrecedence() {"+replay+"} }")
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/BindingReplayTest.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				var message string
				if context == "istest" {
					run := Run(index, Options{SelectedClasses: []string{"BindingReplayTest"}, SelectedMethod: "observeScalarPrecedence", NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("unexpected selection: %+v", run.Summary())
					}
					result := run.Suites[0].Cases[0]
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing observation assertion: %+v; problem=%+v", result, result.Problem)
					}
					message = result.Problem.Message
				} else {
					source, err := os.ReadFile(filepath.Join(directory, "anonymous.apex"))
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(capture), fmt.Sprintf("anonymous_sha256=%x\n", sha256.Sum256(source))) {
						t.Fatal("native anonymous source changed")
					}
					source = []byte(replay)
					analysis := sema.AnalyzeAnonymous(index, string(source), api)
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil || analysis.HasErrors() {
						t.Fatalf("anonymous compile: %v %v", err, analysis.Diagnostics)
					}
					org := orgFromIndex(index)
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					triggers, failures := compileProjectTriggers(index)
					if len(failures) != 0 {
						t.Fatalf("compile triggers: %v", failures)
					}
					for _, trigger := range triggers {
						if err := runner.base.RegisterTrigger(trigger); err != nil {
							t.Fatal(err)
						}
					}
					_, err = runner.newMachine().Execute(program)
					var runtimeErr *vm.RuntimeError
					ok := errors.As(err, &runtimeErr)
					if !ok || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
						t.Fatalf("missing observation assertion: %v", err)
					}
					message = runtimeErr.ExceptionMessage()
				}
				_, payload, ok := strings.Cut(message, "OBS|")
				var got map[string]string
				if !ok || json.Unmarshal([]byte(payload), &got) != nil {
					t.Fatalf("invalid observation: %s", message)
				}
				for _, route := range routes {
					expected := want[route]
					t.Run(route, func(t *testing.T) {
						if actual, present := got[route]; !present || actual != expected {
							t.Fatalf("native=%q; Glade=%q", expected, got[route])
						}
					})
				}
				if len(want) != 6 {
					t.Fatalf("selected native routes=%d, want 6", len(want))
				}
				if len(got) != len(want) {
					t.Fatalf("route count=%d, want %d", len(got), len(want))
				}
			})
		}
	}
}
