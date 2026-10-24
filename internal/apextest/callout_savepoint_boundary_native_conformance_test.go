package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const savepointWindowFixture = pendingWorkFixture + "/savepoint_boundaries"

func savepointWindowNativeRows(t *testing.T, api string) map[string]string {
	t.Helper()
	root := filepath.Join(savepointWindowFixture, "native", "api"+strings.TrimSuffix(api, ".0"), "istest")
	facts, err := os.ReadFile(filepath.Join(root, "RUN-FACTS.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range []string{"api=" + api + "\n", "context=istest\n", "rows=40\n", "native_complete=true\n", "deployment_success=true\n"} {
		if !strings.Contains(string(facts), fact) {
			t.Fatalf("missing native fact %q", fact)
		}
	}
	hashes := 0
	for _, line := range strings.Split(string(facts), "\n") {
		if !strings.HasPrefix(line, "source_sha256=") {
			continue
		}
		name, want, ok := strings.Cut(strings.TrimPrefix(line, "source_sha256="), "|")
		if !ok {
			t.Fatal(line)
		}
		raw, err := os.ReadFile(filepath.Join(savepointWindowFixture, "project", name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(name, "-meta.xml") {
			raw = []byte(strings.ReplaceAll(string(raw), "67.0", api))
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != want {
			t.Fatalf("native source changed: %s", name)
		}
		hashes++
	}
	if hashes != 4 {
		t.Fatalf("got %d native source hashes", hashes)
	}
	raw, err := os.ReadFile(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		key, value, ok := strings.Cut(line, "\t")
		if !ok || rows[key] != "" {
			t.Fatal(line)
		}
		rows[key] = value
	}
	if len(rows) != 40 {
		t.Fatalf("got %d boundary rows", len(rows))
	}
	return rows
}

func TestCalloutSavepointBoundaryNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		rows := savepointWindowNativeRows(t, api)
		raw, err := os.ReadFile(filepath.Join(savepointWindowFixture, "cases.json"))
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct{ ID, Setup, Callout string }
		if err := json.Unmarshal(raw, &cases); err != nil {
			t.Fatal(err)
		}
		source, err := os.ReadFile(filepath.Join(savepointWindowFixture, "project/force-app/main/default/classes/SavepointWindowObservationTest.cls"))
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range cases {
			if rows[row.ID] == "" || !strings.Contains(string(source), "static void "+row.ID+"() {SavepointWindowProbe.mocks();"+row.Setup+"SavepointWindowProbe.emit('"+row.ID+"','"+row.Callout+"');}") {
				t.Fatalf("missing native source or row: %s", row.ID)
			}
			delete(rows, row.ID)
		}
		if len(rows) != 0 {
			t.Fatal("uncovered native rows", rows)
		}
	}
}

func TestCalloutSavepointBoundaryNativeProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api+"/named_istest_and_project_runner_boundaries", func(t *testing.T) {
			root := t.TempDir()
			sourceRoot := filepath.Join(savepointWindowFixture, "project")
			err := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if entry.Name() == ".sf" || entry.Name() == ".sfdx" {
						return fs.SkipDir
					}
					return nil
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
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepted boundary project; diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			want := savepointWindowNativeRows(t, api)
			namedCases := Discover(index, Options{SelectedClasses: []string{"SavepointWindowObservationTest"}})
			namedMethods, methodErrors := compileTestMethodsForIndex(&index, namedCases)
			if len(namedMethods) != len(want) || len(methodErrors) != 0 {
				t.Fatalf("named boundary compilation: methods=%d errors=%v", len(namedMethods), methodErrors)
			}
			t.Run("named_istest", func(t *testing.T) {
				for id, value := range want {
					t.Run(id, func(t *testing.T) {
						program, err := vm.CompileAnonymousWithOptions("SavepointWindowObservationTest."+id+"();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						machine := runner.newMachine()
						for _, method := range namedMethods {
							if err := machine.RegisterMethod(method); err != nil {
								t.Fatal(err)
							}
						}
						machine.EnableTestContext()
						_, err = machine.ExecuteTestInClass(program, "SavepointWindowObservationTest")
						if err == nil || !strings.HasSuffix(err.Error(), "OBSERVED|"+id+"|"+value) {
							t.Errorf("native %q; Glade %v", value, err)
						}
					})
				}
			})
			for _, mode := range []struct {
				name     string
				noCache  bool
				parallel int
			}{{"project_runner_serial", true, 1}, {"project_runner_cached_parallel_cold", false, 2}, {"project_runner_cached_parallel_warm", false, 2}} {
				t.Run(mode.name, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"SavepointWindowObservationTest"}, NoDiskCache: mode.noCache, Parallelism: mode.parallel})
					if run.Summary().Total != len(want) {
						t.Fatalf("unexpected boundary selection %+v", run.Summary())
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
								t.Fatalf("invalid boundary observation %q", payload)
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
						t.Errorf("observed %d/%d boundary rows", len(seen), len(want))
					}
				})
			}
		})
	}
}
