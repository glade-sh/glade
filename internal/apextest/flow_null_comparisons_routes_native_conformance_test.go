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

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the captured numeric Decision matrix through the supported interview,
// record lifecycle, subflow, and committed async adapters of the shared reader.
func TestFlowNullComparisonsRoutesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_null_comparisons"
	const routesFixture = fixture + "/routes"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	caseLines := strings.Split(strings.TrimSpace(read(filepath.Join(fixture, "cases.tsv"))), "\n")
	if len(caseLines) != 61 || caseLines[0] != "id\ttype\tleft\toperator\tright_kind\tright\tbehaviour" {
		t.Fatal("incomplete numeric Decision case table")
	}
	caseIDs := map[string]bool{}
	for _, line := range caseLines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 7 || cells[0] == "" || caseIDs[cells[0]] {
			t.Fatalf("invalid numeric Decision case: %s", line)
		}
		caseIDs[cells[0]] = true
	}
	inputs := map[string]string{}
	for _, name := range []string{"routes.tsv", "anonymous.apex", "enqueue.apex", "readback.apex"} {
		inputs[name] = read(filepath.Join(routesFixture, name))
	}
	routeLines := strings.Split(strings.TrimSpace(inputs["routes.tsv"]), "\n")
	if len(routeLines) != 13 || routeLines[0] != "route\tshape" {
		t.Fatal("incomplete numeric Decision route table")
	}
	routes := []string{}
	routeNames := map[string]bool{}
	for _, line := range routeLines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 2 || cells[0] == "" || cells[1] == "" || routeNames[cells[0]] {
			t.Fatalf("invalid numeric Decision route: %s", line)
		}
		routes = append(routes, cells[0])
		routeNames[cells[0]] = true
	}
	if !routeNames["async"] {
		t.Fatal("missing deferred route")
	}
	// These exact fixture shapes are rejected before Decision evaluation. Keep
	// their native evidence, verify its supported twin, and pin the local boundary
	// without claiming route parity.
	boundaries := map[string]struct{ message, twin string }{
		// invocableFlowActionSupported excludes Currency declarations. The Apex
		// probe JSON-encodes the action errors in its assertion message.
		"action": {
			`unsupported call \"Invocable.Action.invoke hosted Flow action NullCompareAuto\"`,
			"typed",
		},
		// flowStepsBeforeSaveSafeWithTargets permits only root recordCreate;
		// TestFlowTargetReferenceSafetyPreservesLegacyRestrictions pins this.
		"before_delete": {
			"dml: before-delete flow NullRouteDelete contains unsupported side-effect steps",
			"after_insert",
		},
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			files := map[string]string{}
			projectRoot := filepath.Join(routesFixture, "project")
			if err := filepath.WalkDir(projectRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, err := filepath.Rel(projectRoot, path)
				if err != nil {
					return err
				}
				files[filepath.ToSlash(name)] = strings.ReplaceAll(read(path), "62.0", api)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			files["force-app/main/default/flows/NullCompareAuto.flow-meta.xml"] = read(filepath.Join(fixture, "metadata", version, "flows/NullCompareAuto.flow-meta.xml"))
			wants := map[string]map[string]map[string]string{}
			for _, context := range []string{"anonymous", "istest", "committed_anonymous"} {
				contextRoutes := routes
				if context == "anonymous" {
					contextRoutes = nil
					for _, route := range routes {
						if route != "async" {
							contextRoutes = append(contextRoutes, route)
						}
					}
				} else if context == "committed_anonymous" {
					contextRoutes = []string{"async_before", "async"}
				}
				captureRoot := filepath.Join(routesFixture, "native", version, context)
				capture := read(filepath.Join(captureRoot, "RUN-FACTS.txt"))
				count := len(caseIDs) * len(contextRoutes)
				for _, fact := range []string{"api=" + api, "flow_api=" + api, "context=" + context, fmt.Sprintf("rows=%d", count), "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
						t.Fatalf("%s: missing capture fact %s", context, fact)
					}
				}
				for path, body := range files {
					if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(body)))) {
						t.Fatalf("%s: source differs from capture: %s", context, path)
					}
				}
				for name, body := range inputs {
					if !strings.Contains(capture, fmt.Sprintf("input_sha256:%s=%x\n", name, sha256.Sum256([]byte(body)))) {
						t.Fatalf("%s: input differs from capture: %s", context, name)
					}
				}
				rows := strings.Split(strings.TrimSpace(read(filepath.Join(captureRoot, "org.tsv"))), "\n")
				if len(rows) != count+1 || rows[0] != "id\troute\toutcome" {
					t.Fatalf("%s: incomplete native matrix", context)
				}
				want := map[string]map[string]string{}
				for _, route := range contextRoutes {
					want[route] = map[string]string{}
				}
				for _, line := range rows[1:] {
					cells := strings.Split(line, "\t")
					if len(cells) != 3 || !caseIDs[cells[0]] || want[cells[1]] == nil || want[cells[1]][cells[0]] != "" {
						t.Fatalf("%s: invalid native row: %s", context, line)
					}
					pending := cells[1] == "async_before" || context == "istest" && cells[1] == "async"
					if pending && cells[2] != "pending" || !pending && cells[2] != "match" && cells[2] != "other" {
						t.Fatalf("%s: invalid native outcome: %s", context, line)
					}
					want[cells[1]][cells[0]] = cells[2]
				}
				for _, route := range contextRoutes {
					if len(want[route]) != len(caseIDs) {
						t.Fatalf("%s/%s: missing native cases", context, route)
					}
				}
				if context != "committed_anonymous" {
					for route, boundary := range boundaries {
						if !reflect.DeepEqual(want[route], want[boundary.twin]) {
							t.Fatalf("%s/%s: native matrix differs from supported twin %s", context, route, boundary.twin)
						}
					}
				}
				wants[context] = want
			}
			for path, body := range files {
				writeFile(t, filepath.Join(root, filepath.FromSlash(path)), body)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			check := func(t *testing.T, payload, context, route string) {
				t.Helper()
				var got map[string]string
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid observation: %v (%s)", err, payload)
				}
				want := wants[context][route]
				if want == nil {
					t.Fatalf("missing native route %s/%s", context, route)
				}
				if !reflect.DeepEqual(got, want) {
					for key, value := range want {
						if got[key] != value {
							t.Errorf("%s: native %s; Glade %s", key, value, got[key])
						}
					}
					if len(got) != len(want) {
						t.Errorf("observations=%d; want %d", len(got), len(want))
					}
				}
			}
			execute := func(t *testing.T, machine *vm.VM, source, context, route string) vm.Result {
				t.Helper()
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if boundary, ok := boundaries[route]; ok {
					if err == nil || !strings.Contains(err.Error(), boundary.message) {
						t.Fatalf("expected local boundary %q; got %v", boundary.message, err)
					}
					// The local boundary is pinned above; the native numeric
					// matrix for this route is asserted through its twin.
					t.Logf("local route boundary: %s; native numeric matrix covered by %s", boundary.message, boundary.twin)
					return result
				}
				if err != nil {
					t.Fatal(err)
				}
				observed := 0
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "PROBE|"); ok {
						observed++
						check(t, payload, context, route)
					}
				}
				if observed != 1 {
					t.Fatalf("observations=%d; want 1", observed)
				}
				return result
			}
			for _, route := range routes {
				if route != "async" {
					t.Run("anonymous/"+route, func(t *testing.T) {
						execute(t, runner.newMachine(), strings.ReplaceAll(inputs["anonymous.apex"], "ROUTE", route), "anonymous", route)
					})
				}
			}
			for _, temperature := range []string{"cold", "warm"} {
				t.Run("project/"+temperature, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"NullRouteProbeTest"}, NoDiskCache: true, Parallelism: 1})
					if result.Summary().Total != len(routes) || len(result.Suites) != 1 || len(result.Suites[0].Cases) != len(routes) {
						t.Fatalf("observation selection: %s", firstRunProblem(result))
					}
					seen := map[string]bool{}
					for _, row := range result.Suites[0].Cases {
						route := row.MethodName
						if route == "asyncBoundary" {
							route = "async"
						}
						if !routeNames[route] || seen[route] {
							t.Fatalf("unexpected test route: %s", row.MethodName)
						}
						seen[route] = true
						t.Run(route, func(t *testing.T) {
							if boundary, ok := boundaries[route]; ok {
								if row.Problem == nil || !strings.Contains(row.Problem.Message, boundary.message) {
									t.Fatalf("expected local boundary %q; got %+v", boundary.message, row.Problem)
								}
								t.Logf("local route boundary: %s; native numeric matrix covered by %s", boundary.message, boundary.twin)
								return
							}
							if row.Problem == nil || !strings.HasSuffix(row.Problem.Type, "ObservationException") {
								t.Fatalf("%s failed before observation: %+v", row.MethodName, row.Problem)
							}
							_, payload, ok := strings.Cut(row.Problem.Message, "PROBE|")
							if !ok {
								t.Fatalf("missing observation: %+v", row.Problem)
							}
							check(t, payload, "istest", route)
						})
					}
				})
			}
			t.Run("committed_anonymous/async", func(t *testing.T) {
				machine := runner.newMachine()
				result := execute(t, machine, inputs["enqueue.apex"], "committed_anonymous", "async_before")
				if err := machine.DrainAsync(&result); err != nil {
					t.Fatal(err)
				}
				execute(t, machine, inputs["readback.apex"], "committed_anonymous", "async")
			})
		})
	}
}
