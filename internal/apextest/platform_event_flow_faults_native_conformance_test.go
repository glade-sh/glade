package apextest

import (
	"context"
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
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// Flow fault paths: the $Flow.FaultMessage text of a rejected Create Records, a
// failed Update Records, Delete Records and Get Records, each with and without a
// fault connector, in event-triggered, record-triggered and autolaunched flows,
// and the events an interview creates before it faults. Native rows: APIs 62.0
// and 67.0, the anonymous route and the @IsTest deliver() route.
const flowFaultFixture = "testdata/conformance/platform_event_flow_elements/faults"

// Cases that create events also run a second deliver() route: a faulted
// interview's event shows up (or not) only after the next delivery.
type flowFaultCase struct {
	id, kind, operation string
	routes              []string
}

func flowFaultCases(t *testing.T) []flowFaultCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(flowFaultFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 58 || lines[0] != "id\tkind\toperation\troutes\tshape" {
		t.Fatal("incomplete flow fault case table")
	}
	var cases []flowFaultCase
	for _, line := range lines[1:] {
		cells := strings.Split(line, "\t")
		routes := strings.Split(cells[3], ",")
		if len(cells) != 5 || (len(routes) != 2 && len(routes) != 3) || routes[0] != "anonymous" || routes[1] != "deliver" || (len(routes) == 3 && routes[2] != "deliver2") {
			t.Fatalf("invalid flow fault case: %s", line)
		}
		cases = append(cases, flowFaultCase{cells[0], cells[1], cells[2], routes})
	}
	return cases
}

func flowFaultRows(t *testing.T, api string) map[string]map[string]any {
	t.Helper()
	dir := filepath.Join(flowFaultFixture, "native", "api"+strings.TrimSuffix(api, ".0"))
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 127 || lines[0] != "id\troute\tvalue" {
		t.Fatal("incomplete native flow fault route matrix")
	}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		var value map[string]any
		if len(cells) != 3 || json.Unmarshal([]byte(cells[2]), &value) != nil || value == nil || rows[cells[0]+"/"+cells[1]] != nil {
			t.Fatalf("invalid native flow fault row: %s", line)
		}
		rows[cells[0]+"/"+cells[1]] = value
	}
	for _, c := range flowFaultCases(t) {
		for _, route := range c.routes {
			if rows[c.id+"/"+route] == nil {
				t.Fatalf("missing native route: %s/%s", c.id, route)
			}
		}
	}
	return rows
}

func flowFaultProject(t *testing.T, api string) string {
	t.Helper()
	dir := filepath.Join(flowFaultFixture, "native", "api"+strings.TrimSuffix(api, ".0"))
	receipt, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"api=" + api + "\n", "native_complete=true\n", "deployment_success=true\n", "rows=126\n", "org_deleted=true\n"} {
		if !strings.Contains(string(receipt), field) {
			t.Fatalf("incomplete native capture: %s", field)
		}
	}
	for _, name := range []string{"cases.tsv", "anonymous-publish.apex", "anonymous-read.apex"} {
		source, err := os.ReadFile(filepath.Join(flowFaultFixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(receipt), fmt.Sprintf("input_sha256:%s=%x\n", name, sha256.Sum256(source))) {
			t.Fatalf("native input changed: %s", name)
		}
	}
	sourceRoot := filepath.Join(flowFaultFixture, "project")
	root := t.TempDir()
	if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		source := strings.ReplaceAll(string(raw), "62.0", api)
		if !strings.Contains(string(receipt), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(rel), sha256.Sum256([]byte(source)))) {
			t.Errorf("native source changed: %s", rel)
		}
		writeFile(t, filepath.Join(root, rel), source)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// RUN-FACTS.txt: fixture_permission_set_assigned=true.
	configureCapturedFixturePermissions(t, root, flowFaultFixture+"/project/force-app/main/default", "FltFixture")
	return root
}

func TestFlowFaultNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			flowFaultRows(t, api)
			flowFaultProject(t, api)
		})
	}
}

func TestFlowFaultNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := flowFaultRows(t, api)
			index := loadTestIndex(t, flowFaultProject(t, api))
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: []string{"FltProbeTest"}})
			if len(cases) != 69 {
				t.Fatalf("discovered %d methods, want 69", len(cases))
			}
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			methods, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			for _, row := range cases {
				t.Run("named_istest/"+row.MethodName, func(t *testing.T) {
					key := testCaseKey(row)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
					}
					machine := runner.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(methods[key]); err != nil {
						t.Fatal(err)
					}
					_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
					var fault *vm.RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("native observation completed; Glade: %v", err)
					}
					parts := strings.SplitN(row.MethodName, "_", 2)
					prefix := "EVT_OBS|" + parts[0] + "|" + parts[1] + "|"
					_, payload, ok := strings.Cut(fault.ExceptionMessage(), prefix)
					if !ok {
						t.Fatal(fault.ExceptionMessage())
					}
					compareEventExtended(t, payload, want[parts[0]+"/"+parts[1]])
				})
			}
			t.Run("project_test_runner", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"FltProbeTest"}, NoDiskCache: true, Parallelism: 1}, cases)
				results := map[string]testreport.Case{}
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						results[row.MethodName] = row
					}
				}
				for _, row := range cases {
					t.Run(row.MethodName, func(t *testing.T) {
						result, ok := results[row.MethodName]
						if !ok || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("project status=%s; problem=%+v; exclusions=%v", result.Status, result.Problem, report.ExcludedClasses)
						}
						parts := strings.SplitN(row.MethodName, "_", 2)
						_, payload, ok := strings.Cut(result.Problem.Message, "EVT_OBS|"+parts[0]+"|"+parts[1]+"|")
						if !ok {
							t.Fatal(result.Problem.Message)
						}
						compareEventExtended(t, payload, want[parts[0]+"/"+parts[1]])
					})
				}
			})
			t.Run("anonymous_committed", func(t *testing.T) {
				for _, c := range flowFaultCases(t) {
					t.Run(c.id, func(t *testing.T) {
						machine := runner.newMachine()
						execute := func(name string, replacer *strings.Replacer) vm.Result {
							t.Helper()
							source, err := os.ReadFile(filepath.Join(flowFaultFixture, name))
							if err != nil {
								t.Fatal(err)
							}
							program, err := vm.CompileAnonymousWithOptions(replacer.Replace(string(source)), vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							result, err := machine.Execute(program)
							if err != nil {
								t.Fatal(err)
							}
							return result
						}
						result := execute("anonymous-publish.apex", strings.NewReplacer("CASE_ID", c.id, "KIND", c.kind, "OPERATION", c.operation))
						if err := machine.DrainAsync(&result); err != nil {
							t.Fatal(err)
						}
						published := ""
						for _, line := range result.Debug {
							if _, payload, ok := strings.Cut(line, "EVT_PUBLISH|"); ok {
								published = payload
							}
						}
						if published == "" {
							t.Fatal("missing publish observation")
						}
						// The read script receives the publish result as an Apex string literal.
						literal := strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(published)
						result = execute("anonymous-read.apex", strings.NewReplacer("PUBLISHED_JSON", literal, "CASE_ID", c.id))
						for _, line := range result.Debug {
							if _, payload, ok := strings.Cut(line, "EVT_OBS|"+c.id+"|anonymous|"); ok {
								compareEventExtended(t, payload, want[c.id+"/anonymous"])
								return
							}
						}
						t.Fatal("missing anonymous event observation")
					})
				}
			})
		})
	}
}
