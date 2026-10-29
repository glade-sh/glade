package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native null arithmetic outside loops, including an explicitly assigned null
// target, a null Subtract operand, and omitted optional inputs. Screen-less
// processType=Flow observations use Salesforce's Visualforce host; that host
// remains an explicit local boundary. Its native arithmetic is retained here,
// separately from the executable autolaunched route matrix.
func TestFlowLogic2NativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_logic2"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			capture := read("native/" + version + "/RUN-FACTS.txt")
			for _, fact := range []string{"api=" + api, "native_complete=true", "deployment_success=true", "anonymous_rows=36", "istest_rows=36", "visualforce_rows=36", "rejected_rows=1"} {
				if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
					t.Fatalf("missing native fact: %s", fact)
				}
			}
			want := map[string]map[string]any{"anonymous": {}, "istest": {}, "visualforce": {}}
			rows := strings.Split(strings.TrimSuffix(read("native/"+version+"/org.tsv"), "\n"), "\n")
			if len(rows) != 110 || rows[0] != "context\tid\tobservation" {
				t.Fatal("incomplete native matrix")
			}
			for _, line := range rows[1:] {
				cols := strings.SplitN(line, "\t", 3)
				if len(cols) != 3 {
					t.Fatalf("invalid row %q", line)
				}
				if cols[0] == "deployment" {
					if cols[1] != "invalid_logic" || !strings.Contains(cols[2], "undefined filter: 18446744073709551617") {
						t.Fatalf("missing rejection twin: %s", line)
					}
					continue
				}
				values, ok := want[cols[0]]
				if !ok {
					t.Fatalf("unknown context: %s", cols[0])
				}
				if _, exists := values[cols[1]]; exists {
					t.Fatalf("duplicate native row: %s", line)
				}
				var value any
				if err := json.Unmarshal([]byte(cols[2]), &value); err != nil {
					t.Fatal(err)
				}
				values[cols[1]] = value
			}
			for context, values := range want {
				if len(values) != 36 {
					t.Fatalf("%s rows=%d", context, len(values))
				}
			}
			// Both native hosts independently observe the same numeric rule.
			for id, value := range want["visualforce"] {
				auto := strings.Replace(id, "Screenless", "Auto", 1)
				if !reflect.DeepEqual(value, want["anonymous"][auto]) {
					t.Fatalf("native hosts disagree for %s", id)
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, sub := range []string{"sources", "metadata/" + version, "rejection/" + version} {
				err := filepath.WalkDir(filepath.Join(fixture, sub), func(path string, entry fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if entry.IsDir() {
						return nil
					}
					relative, err := filepath.Rel(fixture, path)
					if err != nil {
						return err
					}
					source := read(relative)
					if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(relative), sha256.Sum256([]byte(source)))) {
						t.Fatalf("capture source changed: %s", relative)
					}
					switch {
					case strings.HasPrefix(relative, filepath.FromSlash("sources/classes/")):
						writeFile(t, filepath.Join(root, "force-app/main/default/classes", filepath.Base(path)), source)
						writeFile(t, filepath.Join(root, "force-app/main/default/classes", filepath.Base(path)+"-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
					case strings.HasPrefix(relative, filepath.FromSlash("metadata/"+version+"/flows/")):
						writeFile(t, filepath.Join(root, "force-app/main/default/flows", filepath.Base(path)), source)
					}
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Run("rejected_metadata", func(t *testing.T) {
				path := filepath.Join(fixture, "rejection", version, "flows", "ArithmeticInvalidLogic.flow-meta.xml")
				index, err := automation.LoadProject(project.Project{FlowFiles: []string{path}})
				if err != nil {
					t.Fatal(err)
				}
				if len(index.Diagnostics) != 1 || index.Diagnostics[0].Code != "GLADEAUTO002" || index.Diagnostics[0].File != path || !strings.Contains(index.Diagnostics[0].Message, "start filter logic") {
					t.Fatalf("native rejection did not warn: %v", index.Diagnostics)
				}
				if len(index.Flows) != 1 || len(index.Flows[0].Rules) != 1 {
					t.Fatalf("native rejection fallback: %#v", index.Flows)
				}
				rule := index.Flows[0].Rules[0]
				if rule.APIVersion != api || rule.Formula != "" || len(rule.Criteria) != 2 || len(rule.CriteriaGroups) != 0 {
					t.Fatalf("native rejection did not retain AND fallback: %#v", rule)
				}
			})
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted project: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, context := range []string{"anonymous", "istest"} {
				t.Run(context, func(t *testing.T) {
					machine := runner.newMachine()
					if context == "istest" {
						machine.EnableTestContext()
					}
					program, err := vm.CompileAnonymousWithOptions(`System.assert(false,'PROBE|'+JSON.serialize(ArithmeticProbe.observe()));`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(program)
					checkFlowLogic2Payload(t, err, want[context])
				})
			}
			// The second project run reuses the runtime cache for the same index.
			for _, route := range []string{"project", "warm"} {
				t.Run(route, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"ArithmeticProbeTest"}, SelectedMethod: "observe", Parallelism: 1})
					if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("observation route: %s", firstRunProblem(result))
					}
					checkFlowLogic2Message(t, result.Suites[0].Cases[0].Problem.Message, want["istest"])
				})
			}
		})
	}
}

func checkFlowLogic2Payload(t *testing.T, err error, want map[string]any) {
	t.Helper()
	if err == nil {
		t.Fatal("missing observation assertion")
	}
	checkFlowLogic2Message(t, err.Error(), want)
}

func checkFlowLogic2Message(t *testing.T, message string, want map[string]any) {
	t.Helper()
	_, payload, ok := strings.Cut(message, "PROBE|")
	if !ok {
		t.Fatalf("missing observation payload: %s", message)
	}
	// VM errors may append the source location after the assertion message.
	var got map[string]any
	if err := json.NewDecoder(strings.NewReader(payload)).Decode(&got); err != nil {
		t.Fatalf("invalid observation: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("observations=%d, native=%d", len(got), len(want))
	}
	for id, value := range want {
		if !reflect.DeepEqual(got[id], value) {
			t.Errorf("%s: Glade=%#v, native=%#v", id, got[id], value)
		}
	}
}
