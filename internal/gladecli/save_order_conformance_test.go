package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestSaveOrderFixtureDoesNotEnterImplicitProject(t *testing.T) {
	p, err := project.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range p.FlowFiles {
		if strings.Contains(filepath.ToSlash(file), "/testdata/save_order/") {
			t.Fatalf("save-order fixture discovered as implicit project Flow: %s", file)
		}
	}
}

// Each native row records the observation returned by an intentional exception
// or assertion. Compare its full payload, including the route and saved state.
func TestSaveOrderNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	replaySaveOrder(t, "depth", "depth", "org.tsv", "depth.apex", "DepthTest", "boundary", "DepthResultException")
	replaySaveOrder(t, "combined", "combined", "org.tsv", "anonymous.apex", "SaveOrderTest", "combined", "ProbeResultException")
	replaySaveOrder(t, "rollup", "rollup", "org.tsv", "anonymous.apex", "RollupTest", "combined", "RollupResultException")
	replaySaveOrder(t, "rollup", "control", "siblings.tsv", "siblings.apex", "RollupControlTest", "combined", "RollupResultException")
	replaySaveOrder(t, "rollup_events", "single", "single.tsv", "single.apex", "RollupSideSingle", "probe", "RollupResultException")
	replaySaveOrder(t, "rollup_events", "mixed", "mixed.tsv", "mixed.apex", "RollupSideMixed", "probe", "RollupResultException")
	replaySaveOrder(t, "retry_validation", "single", "single.tsv", "single.apex", "RetrySingle", "probe", "RollupResultException")
	replaySaveOrder(t, "retry_validation", "partial", "partial.tsv", "partial.apex", "RetryPartial", "probe", "RollupResultException")
	replaySaveOrder(t, "retry_validation", "atomic", "atomic.tsv", "atomic.apex", "RetryAtomic", "probe", "RollupResultException")
}

func replaySaveOrder(t *testing.T, fixture, rowID, table, script, classPrefix, method, exception string) {
	t.Helper()
	dir := filepath.Join("testdata", "save_order", fixture)
	raw, err := os.ReadFile(filepath.Join(dir, table))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	header := "id\tapi\troute\tcompiled\ttransport\tobservation"
	if fixture == "depth" {
		header += " (newlines escaped)"
	}
	if len(lines) != 5 || lines[0] != header {
		t.Fatalf("%s: expected the four native API and route observations", fixture)
	}
	source, err := os.ReadFile(filepath.Join(dir, script))
	if err != nil {
		t.Fatal(err)
	}
	for i, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		projectFiles := filepath.Join(root, "force-app")
		if err := os.CopyFS(projectFiles, os.DirFS(filepath.Join(dir, "force-app"))); err != nil {
			t.Fatal(err)
		}
		project := fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api)
		writeTestFile(t, filepath.Join(root, "sfdx-project.json"), project)
		if fixture == "rollup" || fixture == "rollup_events" || fixture == "retry_validation" {
			// The scratch org assigned this deployed permission set before running Apex.
			writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Probe_Access]\n")
		}
		for j, route := range []string{"anonymous", "istest"} {
			t.Run(fixture+"/"+rowID+"/"+api+"/"+route, func(t *testing.T) {
				row := strings.Split(lines[1+2*i+j], "\t")
				if len(row) != 6 || row[0] != rowID || row[1] != api || row[2] != route || row[3] != "true" {
					t.Fatalf("unexpected capture row %q", row)
				}
				var stdout, stderr bytes.Buffer
				if route == "anonymous" {
					checkAnonymousSave(t, root, source, exception, row[5])
					return
				}
				class := classPrefix + strings.TrimSuffix(api, ".0")
				args := []string{"test", "--project", root, "--no-serve", "--class", class, "--method", method, "--json"}
				code := Run(context.Background(), args, &stdout, &stderr)
				run, err := decodeTestRunJSON(stdout.Bytes())
				if err != nil || code != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one assertion, exit %d: %v %s %s", code, err, stderr.String(), stdout.String())
				}
				c := run.Suites[0].Cases[0]
				want := "Assertion Failed: Expected: EXPECTED, Actual: " + row[5]
				if c.ClassName != class || c.MethodName != method || c.Problem == nil || c.Problem.Type != "System.AssertException" || saveOrderObservation(t, exception, c.Problem.Message) != saveOrderObservation(t, exception, want) {
					t.Fatalf("Glade %s.%s: %+v\nnative: %s", c.ClassName, c.MethodName, c.Problem, want)
				}
			})
		}
		if rowID == "rollup" {
			controls, err := os.ReadFile(filepath.Join(dir, "controls.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(controls)), "\n")
			if len(rows) != 5 || rows[0] != "case\tapi\tanonymous_observation" {
				t.Fatal("expected both single-parent controls at both APIs")
			}
			for n, name := range []string{"single-ok", "single-reject"} {
				t.Run(fixture+"/"+api+"/"+name, func(t *testing.T) {
					row := strings.Split(rows[1+2*n+i], "\t")
					if len(row) != 3 || row[0] != name || row[1] != api {
						t.Fatalf("unexpected control row %q", row)
					}
					source, err := os.ReadFile(filepath.Join(dir, name+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					checkAnonymousSave(t, root, source, exception, row[2])
				})
			}
		}
	}
}

func checkAnonymousSave(t *testing.T, root string, source []byte, exception, observation string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	args := []string{"exec", "--project", root, "--json", string(source)}
	code := Run(context.Background(), args, &stdout, &stderr)
	want := "glade: " + exception + ": " + observation
	if got := strings.TrimSpace(stderr.String()); code == 0 || saveOrderObservation(t, exception, got) != saveOrderObservation(t, exception, want) {
		t.Fatalf("exit %d\nGlade: %s\nnative: %s", code, got, want)
	}
}

// Compare the trigger-depth error code, entry count, full trace and stored
// row count. The native nested diagnostic is preserved in the captured row.
func saveOrderObservation(t *testing.T, exception, value string) string {
	t.Helper()
	if exception != "DepthResultException" {
		return value
	}
	const code = "|17:false:CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY:"
	before, messageAndState, ok := strings.Cut(value, code)
	if !ok {
		t.Fatalf("missing depth error in %q", value)
	}
	message, state, ok := strings.Cut(messageAndState, ":entries=")
	if !ok || message == "" {
		t.Fatalf("missing depth diagnostic or entry count in %q", value)
	}
	return before + code + ":entries=" + state
}
