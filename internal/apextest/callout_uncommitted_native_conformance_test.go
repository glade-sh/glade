package apextest

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const pendingWorkFixture = "testdata/conformance/callout_uncommitted_work"

func pendingWorkNativeRows(t *testing.T, api, route string) map[string]string {
	t.Helper()
	root := filepath.Join(pendingWorkFixture, "native", "api"+strings.TrimSuffix(api, ".0"), route)
	facts, err := os.ReadFile(filepath.Join(root, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []string{"api=" + api + "\n", "context=" + route + "\n", "rows=17\n", "native_complete=true\n", "deployment_success=true\n"} {
		if !strings.Contains(string(facts), fact) {
			t.Fatalf("missing native fact %q", fact)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		key, value, ok := strings.Cut(line, "\t")
		if !ok || rows[key] != "" {
			t.Fatalf("invalid row %q", line)
		}
		rows[key] = value
	}
	if len(rows) != 17 {
		t.Fatalf("got %d native rows", len(rows))
	}
	return rows
}

func TestCalloutUncommittedNativeRows(t *testing.T) {
	t.Run("test_boundaries", checkCalloutUncommittedBoundaryNativeRows)
	raw, err := os.ReadFile(filepath.Join(pendingWorkFixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 18 || lines[0] != "id\tshape\troutes\tapis" {
		t.Fatal("incomplete callout case table")
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				rows := pendingWorkNativeRows(t, api, route)
				for _, line := range lines[1:] {
					cells := strings.Split(line, "\t")
					if len(cells) != 4 || cells[1] == "" || cells[2] != "anonymous,istest" || cells[3] != "62.0,67.0" || rows[cells[0]] == "" {
						t.Fatalf("invalid or unobserved case %q", line)
					}
					delete(rows, cells[0])
				}
				if len(rows) != 0 {
					t.Fatal("native capture contains an unknown case")
				}
			})
		}
	}
}

func TestCalloutUncommittedNativeConformance(t *testing.T) {
	t.Run("test_boundaries", checkCalloutUncommittedBoundaryNativeConformance)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(pendingWorkFixture, "project")
			if err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err == nil && entry.IsDir() && (entry.Name() == ".sf" || entry.Name() == ".sfdx") {
					return fs.SkipDir
				}
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(raw), "67.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native project accepted; diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			t.Run("anonymous", func(t *testing.T) {
				wantRows := pendingWorkNativeRows(t, api, "anonymous")
				// The qualified invocation twins the captured lexical UW10 route.
				wantRows["UW10Qualified"] = wantRows["UW10"]
				for id, want := range wantRows {
					t.Run(id, func(t *testing.T) {
						switch id {
						case "UW02", "UW07", "UW12":
							t.Logf("Native hosted endpoint rejection retained as a transport boundary observation: %s", want)
							return
						case "UW03", "UW04":
							t.Logf("Native test-only entry rejection retained outside callout transaction conformance: %s", want)
							return
						}
						invocation := "PendingWorkProbe.observe('" + id + "')"
						if id == "UW10Qualified" {
							invocation = "PendingWorkProbe.qualifiedFuture()"
						}
						program, err := vm.CompileAnonymousWithOptions("System.debug('OBSERVED|"+id+"|'+"+invocation+");", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := runner.newMachine().Execute(program)
						if strings.HasPrefix(want, "uncaught|") {
							var fault *vm.RuntimeError
							if !errors.As(err, &fault) || "uncaught|"+fault.Type+"|"+fault.ExceptionMessage() != want {
								t.Errorf("native %q; Glade error %v", want, err)
							}
							return
						}
						if err != nil {
							t.Fatal(err)
						}
						got := ""
						for _, line := range result.Debug {
							if _, value, ok := strings.Cut(line, "OBSERVED|"+id+"|"); ok {
								got = value
							}
						}
						if got != want {
							t.Errorf("native %q; Glade %q", want, got)
						}
					})
				}
			})
			t.Run("project_istest", func(t *testing.T) {
				want := pendingWorkNativeRows(t, api, "istest")
				want["UW10Qualified"] = want["UW10"]
				run := Run(index, Options{SelectedClasses: []string{"PendingWorkObservationTest"}, NoDiskCache: true, Parallelism: 1})
				if run.Summary().Total != len(want) {
					t.Fatalf("unexpected project selection: %+v excluded=%+v", run.Summary(), run.ExcludedClasses)
				}
				seen := map[string]bool{}
				for _, suite := range run.Suites {
					for _, row := range suite.Cases {
						if row.Problem == nil {
							t.Errorf("%s: missing observation", row.Name)
							continue
						}
						_, payload, ok := strings.Cut(row.Problem.Message, "OBSERVED|")
						if !ok {
							t.Errorf("%s: %s", row.Name, row.Problem.Message)
							continue
						}
						id, value, ok := strings.Cut(payload, "|")
						if !ok || want[id] == "" || seen[id] {
							t.Fatalf("invalid project observation %q", payload)
						}
						seen[id] = true
						t.Run(id, func(t *testing.T) {
							if value != want[id] {
								t.Errorf("native %q; Glade %q", want[id], value)
							}
						})
					}
				}
				if len(seen) != len(want) {
					t.Errorf("observed %d/%d project rows", len(seen), len(want))
				}
			})
		})
	}
}
