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

// A flow that declares only Number variables runs through Invocable.Action
// without any Currency declaration. Its native rows must equal the Number rows
// of the full numeric Decision matrix captured on the same action route.
func TestFlowNullComparisonsNumberOnlyActionNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_null_comparisons"
	const twin = fixture + "/number_only_action"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	rowsOf := func(path string) map[string]string {
		t.Helper()
		lines := strings.Split(strings.TrimSpace(read(path)), "\n")
		if lines[0] != "id\troute\toutcome" {
			t.Fatalf("%s: bad header", path)
		}
		rows := map[string]string{}
		for _, line := range lines[1:] {
			cells := strings.Split(line, "\t")
			if len(cells) != 3 {
				t.Fatalf("%s: invalid row %q", path, line)
			}
			if cells[1] != "action" || !strings.HasPrefix(cells[0], "N") {
				continue
			}
			if rows[cells[0]] != "" {
				t.Fatalf("%s: duplicate row %q", path, line)
			}
			rows[cells[0]] = cells[2]
		}
		return rows
	}
	probe := read(filepath.Join(twin, "anonymous.apex"))
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			files := map[string]string{}
			projectRoot := filepath.Join(twin, "project")
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
			files["force-app/main/default/flows/NullNumAuto.flow-meta.xml"] = read(filepath.Join(twin, "metadata", version, "flows/NullNumAuto.flow-meta.xml"))
			if strings.Contains(files["force-app/main/default/flows/NullNumAuto.flow-meta.xml"], "Currency") {
				t.Fatal("twin flow declares Currency")
			}
			wants := map[string]map[string]string{}
			for _, context := range []string{"anonymous", "istest"} {
				capture := read(filepath.Join(twin, "native", version, context, "RUN-FACTS.txt"))
				for _, fact := range []string{"api=" + api, "flow_api=" + api, "context=" + context, "rows=30", "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
						t.Fatalf("%s: missing capture fact %s", context, fact)
					}
				}
				for path, body := range files {
					if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(body)))) {
						t.Fatalf("%s: source differs from capture: %s", context, path)
					}
				}
				if !strings.Contains(capture, fmt.Sprintf("input_sha256:anonymous.apex=%x\n", sha256.Sum256([]byte(probe)))) {
					t.Fatalf("%s: input differs from capture", context)
				}
				want := rowsOf(filepath.Join(twin, "native", version, context, "org.tsv"))
				if len(want) != 30 {
					t.Fatalf("%s: incomplete native rows", context)
				}
				// The Number-only flow must answer exactly like the Number rows
				// of the full flow on the same route.
				full := rowsOf(filepath.Join(fixture, "routes", "native", version, context, "org.tsv"))
				for id, outcome := range want {
					if full[id] != outcome {
						t.Fatalf("%s/%s: twin %s; full flow %s", context, id, outcome, full[id])
					}
				}
				wants[context] = want
			}
			root := t.TempDir()
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
			check := func(t *testing.T, payload, context string) {
				t.Helper()
				var got map[string]string
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid observation: %v (%s)", err, payload)
				}
				if !reflect.DeepEqual(got, wants[context]) {
					for id, outcome := range wants[context] {
						if got[id] != outcome {
							t.Errorf("%s: native %s; Glade %s", id, outcome, got[id])
						}
					}
					t.Errorf("observations=%d; want %d", len(got), len(wants[context]))
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(probe, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				observed := 0
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "PROBE|"); ok {
						observed++
						check(t, payload, "anonymous")
					}
				}
				if observed != 1 {
					t.Fatalf("observations=%d; want 1", observed)
				}
			})
			t.Run("istest", func(t *testing.T) {
				result := Run(index, Options{SelectedClasses: []string{"NullNumActionProbeTest"}, NoDiskCache: true, Parallelism: 1})
				if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
					t.Fatalf("observation selection: %s", firstRunProblem(result))
				}
				row := result.Suites[0].Cases[0]
				if row.Problem == nil || !strings.HasSuffix(row.Problem.Type, "ObservationException") {
					t.Fatalf("failed before observation: %+v", row.Problem)
				}
				_, payload, ok := strings.Cut(row.Problem.Message, "PROBE|")
				if !ok {
					t.Fatalf("missing observation: %+v", row.Problem)
				}
				check(t, payload, "istest")
			})
		})
	}
}
