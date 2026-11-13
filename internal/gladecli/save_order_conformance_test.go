package gladecli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each native row records the observation returned by an intentional exception
// or assertion. Compare its full payload, including the route and saved state.
func TestSaveOrderNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	replaySaveOrder(t, "combined", "combined", "org.tsv", "anonymous.apex", "SaveOrderTest", "combined", "ProbeResultException")
	replaySaveOrder(t, "rollup", "rollup", "org.tsv", "anonymous.apex", "RollupTest", "combined", "RollupResultException")
	replaySaveOrder(t, "rollup", "control", "siblings.tsv", "siblings.apex", "RollupControlTest", "combined", "RollupResultException")
	replaySaveOrder(t, "rollup_events", "single", "single.tsv", "single.apex", "RollupSideSingle", "probe", "RollupResultException")
	replaySaveOrder(t, "rollup_events", "mixed", "mixed.tsv", "mixed.apex", "RollupSideMixed", "probe", "RollupResultException")
}

func replaySaveOrder(t *testing.T, fixture, rowID, table, script, classPrefix, method, exception string) {
	t.Helper()
	dir := filepath.Join("testdata", "save_order", fixture)
	raw, err := os.ReadFile(filepath.Join(dir, table))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 5 || lines[0] != "id\tapi\troute\tcompiled\ttransport\tobservation" {
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
		if fixture == "rollup" || fixture == "rollup_events" {
			// The scratch org assigned this deployed permission set before running Apex.
			writeTestFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Probe_Access]\n")
		}
		for j, route := range []string{"anonymous", "istest"} {
			t.Run(rowID+"/"+api+"/"+route, func(t *testing.T) {
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
				if c.ClassName != class || c.MethodName != method || c.Problem == nil || c.Problem.Type != "System.AssertException" || c.Problem.Message != want {
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
	if got := strings.TrimSpace(stderr.String()); code == 0 || got != want {
		t.Fatalf("exit %d\nGlade: %s\nnative: %s", code, got, want)
	}
}
