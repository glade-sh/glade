package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRestoredChildRelationshipCacheNativeProjectRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/child_relationship_cache"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, class := range []string{"ChildCacheProbe", "ChildCacheProbeTest"} {
				body, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes", class+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls"), string(body))
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			_, runtime, err := runtimeFromIndexWithSourceDigests(index, nil, newSourceCache())
			if err != nil || runtime.BaseErr != nil {
				t.Fatalf("project runtime: %v %v", err, runtime.BaseErr)
			}
			nativeDir := filepath.Join(fixture, "native", "api"+api[:2])
			receipt, err := os.ReadFile(filepath.Join(nativeDir, "RUN-FACTS.txt"))
			if err != nil || !strings.Contains(string(receipt), "native_complete=true\n") || !strings.Contains(string(receipt), "source_api="+api+"\n") {
				t.Fatalf("incomplete native capture: %v", err)
			}
			raw, err := os.ReadFile(filepath.Join(nativeDir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			table := strings.Split(strings.TrimSpace(string(raw)), "\n")
			if len(table) != 3 || table[0] != "source_api\tentry\troute\tobservation" {
				t.Fatalf("native rows = %d, want header and two observations", len(table))
			}
			check := func(t *testing.T, observation, expected string) {
				t.Helper()
				var got, want map[string]any
				if err := json.Unmarshal([]byte(observation), &got); err != nil {
					t.Fatalf("invalid observation %q: %v", observation, err)
				}
				if err := json.Unmarshal([]byte(expected), &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("Glade=%v Salesforce=%v", got, want)
				}
				t.Logf("observation=%s", observation)
			}
			for _, line := range table[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) != 4 || row[0] != api || row[2] != "observe" || (row[1] != "anonymous" && row[1] != "istest") {
					t.Fatalf("invalid native row: %v", row)
				}
				for clone := 0; clone < 2; clone++ {
					t.Run(fmt.Sprintf("%s/restored-clone-%d", row[1], clone), func(t *testing.T) {
						org := runtime.restored.CloneOrg()
						generation := org.RuntimeGenerationID()
						machine := runtime.restored.CloneMachine(nil)
						machine.SetRuntimeTemplateOrg(&org)
						if org.RuntimeGenerationID() != generation {
							t.Error("project route discarded the restored schema generation")
						}
						body := "System.debug('CHILD_CACHE|'+ChildCacheProbe.observe());"
						if row[1] == "istest" {
							machine.EnableTestContext()
							cases := Discover(index, Options{SelectedClasses: []string{"ChildCacheProbeTest"}})
							methods, errs := compileTestMethods(cases)
							if len(cases) != 1 || len(errs) != 0 {
								t.Fatalf("named compilation: %v %v", cases, errs)
							}
							if err := machine.RegisterMethod(methods[testCaseKey(cases[0])]); err != nil {
								t.Fatal(err)
							}
							programs, errs := compileTestInvokePrograms(cases)
							if len(errs) != 0 {
								t.Fatal(errs)
							}
							_, err := machine.ExecuteInClass(programs[testCaseKey(cases[0])], "ChildCacheProbeTest")
							check(t, childCacheObservation(t, fmt.Sprint(err)), row[3])
						} else {
							program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil || len(result.Debug) != 1 {
								t.Fatalf("anonymous: %v %v", result.Debug, err)
							}
							check(t, childCacheObservation(t, result.Debug[0]), row[3])
							value, err := machine.CallStatic("ChildCacheProbe.observe", nil)
							if err != nil {
								t.Fatal(err)
							}
							check(t, value.Text, row[3])
						}
					})
				}
				if row[1] == "istest" {
					t.Run("test-runner", func(t *testing.T) {
						run := Run(index, Options{SelectedClasses: []string{"ChildCacheProbeTest"}, NoDiskCache: true, Parallelism: 1})
						if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("runner cases: %#v", run.Summary())
						}
						result := run.Suites[0].Cases[0]
						if result.Status != testreport.StatusFail {
							t.Fatalf("expected observation assertion: %#v", result)
						}
						if result.Problem == nil {
							t.Fatal("runner omitted observation assertion")
						}
						check(t, childCacheObservation(t, result.Problem.Message), row[3])
					})
				}
			}
		})
	}
}

func childCacheObservation(t *testing.T, message string) string {
	t.Helper()
	_, observation, found := strings.Cut(message, "CHILD_CACHE|")
	if !found {
		t.Fatalf("missing child relationship observation: %s", message)
	}
	var value json.RawMessage
	if err := json.NewDecoder(strings.NewReader(observation)).Decode(&value); err != nil {
		t.Fatal(err)
	}
	return string(value)
}
