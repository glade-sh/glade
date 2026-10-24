package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"errors"
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

func TestFlowEntryMatrixNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/flow_entry_matrix"
	capture, err := os.ReadFile(filepath.Join(directory, "RUN-FACTS.txt"))
	if err != nil || !strings.Contains(string(capture), "native_complete=true\n") {
		t.Fatalf("incomplete native capture: %v", err)
	}
	file, err := os.Open(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 313 || strings.Join(rows[0], "\t") != "source_api\troute\tphase\tevent\tentry_mode\toperator\tdml_route\tobservation" {
		t.Fatalf("incomplete native matrix: %d rows, %v", len(rows), err)
	}
	type stages = map[string][]int
	want := map[string]map[string]stages{}
	for _, row := range rows[1:] {
		if len(row) != 8 {
			t.Fatal("invalid native row")
		}
		group := strings.Join(row[2:5], "_")
		key := row[0] + "/" + row[1] + "/" + group
		cell := strings.Join(row[2:7], "/")
		if want[key] == nil {
			want[key] = map[string]stages{}
		}
		var value stages
		if _, duplicate := want[key][cell]; duplicate || json.Unmarshal([]byte(row[7]), &value) != nil {
			t.Fatalf("invalid or duplicate native cell: %s/%s", key, cell)
		}
		want[key][cell] = value
	}
	groups := []string{"before_create_always", "before_update_always", "after_create_always", "after_update_always", "before_update_changed", "after_update_changed"}
	if len(want) != 24 {
		t.Fatalf("native route groups=%d, want 24", len(want))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			projectDirectory := filepath.Join(directory, "api"+strings.TrimSuffix(api, ".0"))
			err := filepath.WalkDir(projectDirectory, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(projectDirectory, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(name), sha256.Sum256(body))) {
					t.Fatalf("native source changed: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, group := range groups {
				t.Run("anonymous/"+group, func(t *testing.T) {
					source, err := os.ReadFile(filepath.Join(root, "anonymous-"+group+".apex"))
					if err != nil {
						t.Fatal(err)
					}
					program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					_, execErr := runner.execute(program)
					var runtimeErr *vm.RuntimeError
					if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
						t.Fatalf("missing native observation assertion: %v", execErr)
					}
					checkFlowEntryMatrix(t, group, runtimeErr.ExceptionMessage(), want[api+"/anonymous/"+group])
				})
			}
			t.Run("istest", func(t *testing.T) {
				run := Run(index, Options{SelectedClasses: []string{"CedarEntryObservationTest"}, NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != len(groups) {
					t.Fatalf("unexpected project test selection: %+v", run.Summary())
				}
				seen := map[string]bool{}
				for _, result := range run.Suites[0].Cases {
					t.Run(result.MethodName, func(t *testing.T) {
						if seen[result.MethodName] || result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
							t.Fatalf("missing or repeated native observation assertion: %+v", result)
						}
						seen[result.MethodName] = true
						checkFlowEntryMatrix(t, result.MethodName, result.Problem.Message, want[api+"/istest/"+result.MethodName])
					})
				}
			})
		})
	}
}

func checkFlowEntryMatrix(t *testing.T, group, message string, want map[string]map[string][]int) {
	t.Helper()
	_, payload, found := strings.Cut(message, "ENTRY_MATRIX|"+group+"|")
	var got map[string]map[string][]int
	if !found || json.Unmarshal([]byte(payload), &got) != nil {
		t.Fatalf("invalid observation: %s", message)
	}
	cells := 15
	if strings.HasSuffix(group, "_changed") {
		cells = 9
	}
	if len(want) != cells || len(got) != cells {
		t.Fatalf("native/Glade matrix cells=%d/%d, want %d", len(want), len(got), cells)
	}
	operators := []string{"EqualTo", "NotEqualTo", "GreaterThan", "IsNull", "IsChanged"}
	if cells == 9 {
		operators = []string{"NotEqualTo", "GreaterThan", "IsNull"}
	}
	for _, operator := range operators {
		for _, route := range []string{"apex_dml", "database", "bulk_dml"} {
			cell := strings.ReplaceAll(group, "_", "/") + "/" + operator + "/" + route
			t.Run(cell, func(t *testing.T) {
				stages := want[cell]
				stage := "created"
				if strings.Contains(group, "_update_") {
					stage = "updated"
				}
				fired := []int{1, 0}
				if route == "bulk_dml" {
					fired = []int{1, 1, 0}
				}
				if !reflect.DeepEqual(stages[stage], fired) {
					t.Fatalf("native matching/control side effects=%v, want %v", stages[stage], fired)
				}
				if !reflect.DeepEqual(got[cell], stages) {
					t.Fatalf("saved Flow effects: native=%v; Glade=%v", stages, got[cell])
				}
			})
		}
	}
}
