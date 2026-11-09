package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestNoRowsHierarchyDisabledNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/no_rows_visibility/disabled-project"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("invalid observations: %v", err)
	}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[1] != "disabled" || row[2] != "hierarchy" {
			t.Fatal("invalid row")
		}
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"PrivateSetupVisibilityTest"}, SelectedMethod: "hierarchy", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED|hierarchy|")
			var got, want map[string]any
			if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", row[3], payload)
			}
		})
	}
}

func TestNoRowsVisibilityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "no_rows_visibility", "PrivateSetupVisibilityTest", []string{"62.0", "67.0"}, 11)
}

func TestNoRowsHierarchyCacheTransitions(t *testing.T) {
	const project = "testdata/conformance/no_rows_visibility/project"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, step := range []struct {
				config string
				parent float64
			}{
				{"org:\n  hierarchyAccessObjects: []\n", 0},
				{"org:\n  hierarchyAccessObjects: [PrivateProbe__c]\n", 1},
				{"org:\n  hierarchyAccessObjects: []\n", 0},
			} {
				writeFile(t, filepath.Join(root, "glade.yml"), step.config)
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PrivateSetupVisibilityTest"}, SelectedMethod: "hierarchy", Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("missing observation: %+v", run.Summary())
				}
				_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED|hierarchy|")
				var got map[string]any
				if !ok || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, map[string]any{"parent": step.parent, "unrelated": float64(0)}) {
					t.Fatalf("parent=%v; Glade=%s", step.parent, payload)
				}
			}
		})
	}
}

func TestNoRowsLocatorAnonymousNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/no_rows_visibility"
	raw, err := os.ReadFile(filepath.Join(dir, "anonymous.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(dir, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 3 || rows[0] != "source_api\tmethod\tobservation" {
		t.Fatal("incomplete observations")
	}
	for _, row := range rows[1:] {
		fields := strings.Split(row, "\t")
		if len(fields) != 3 || fields[1] != "anonymous" {
			t.Fatal("invalid observation")
		}
		t.Run(fields[0], func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "62.0", fields[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = fields[0]
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: fields[0]})
			if err != nil {
				t.Fatal(err)
			}
			_, err = runner.newMachine().Execute(program)
			var observation *vm.RuntimeError
			if !errors.As(err, &observation) || !strings.HasSuffix(observation.ExceptionMessage(), "OBSERVED|anonymous|"+fields[2]) {
				t.Fatalf("native locator observation %s, got %v", fields[2], err)
			}
		})
	}
}

// Warm project twins of no_rows_visibility/org.tsv:18-21.
func TestNoRowsControlledDetailCache(t *testing.T) {
	const dir = "testdata/conformance/no_rows_visibility"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[1] != "controlledDetail" && row[1] != "controlledHierarchy" {
			continue
		}
		api, method, expected := row[0], row[1], row[2]
		t.Run(api+"/"+method, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(dir, "project")
			err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(expected), &want); err != nil {
				t.Fatal(err)
			}
			for _, pass := range []string{"cold", "warm"} {
				t.Run(pass, func(t *testing.T) {
					run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PrivateSetupVisibilityTest"}, SelectedMethod: method, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("missing observation: %+v", run.Summary())
					}
					_, payload, ok := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "OBSERVED|"+method+"|")
					var got map[string]any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil || !reflect.DeepEqual(got, want) {
						t.Fatalf("native=%s; Glade=%s", expected, payload)
					}
				})
			}
		})
	}
}
