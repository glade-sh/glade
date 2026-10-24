package apextest

import (
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

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFeatureParameterNativeConformance(t *testing.T) {
	index := loadTestIndex(t, "testdata/conformance/feature_parameters/project")
	org, err := orgFromIndexWithError(index)
	if err != nil {
		t.Fatal(err)
	}
	if got := featureParameterNativeCount(t, org.Metadata); got != 4 {
		t.Errorf("loaded feature parameter count = %d, want 4", got)
	}
	runInnerTypeNativeConformance(t, "feature_parameters", "FeatureParameterObservationTest", "62.0", "67.0")
}

func TestFeatureParameterSetNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/feature_parameters"
	file, err := os.Open(filepath.Join(fixture, "operations.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 || strings.Join(rows[0], ",") != "source_api,operation,observation" {
		t.Fatalf("incomplete feature parameter operation matrix: rows=%d", len(rows))
	}

	var api string
	var machine *vm.VM
	var runner *conformanceRunner
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("invalid feature parameter operation row: %v", row)
		}
		if row[0] != api {
			api = row[0]
			root := copyFeatureParameterProject(t, fixture, api)
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner = newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine = runner.newMachine()
		}
		if row[1] == "read" {
			runner.org.Org.Metadata = machine.Org.Metadata
			machine = runner.newMachine()
		}
		t.Run(api+"/"+row[1], func(t *testing.T) {
			var want map[string]any
			if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
				t.Fatal(err)
			}
			program, err := featureParameterOperationProgram(fixture, row[1], api)
			if err != nil {
				t.Fatal(err)
			}
			executionMachine := machine
			if row[1] == "null_set" || row[1] == "locked_set" {
				executionMachine = runner.newMachine()
			}
			result, err := executionMachine.Execute(program)
			if expected, ok := want["exception"].(string); ok {
				var runtimeErr *vm.RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != expected {
					t.Fatalf("error = %#v, want %s", err, expected)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			observed := ""
			for _, line := range result.Debug {
				if at := strings.Index(line, "OBS|"); at >= 0 {
					observed = line[at+len("OBS|"):]
					break
				}
			}
			var got map[string]any
			if observed == "" || json.Unmarshal([]byte(observed), &got) != nil {
				t.Fatalf("invalid operation observation %q", observed)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%v; Glade=%v", want, got)
			}
		})
	}
}

func copyFeatureParameterProject(t *testing.T, fixture, api string) string {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(fixture, "project")
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
	return root
}

func featureParameterOperationProgram(fixture, operation, api string) (ir.Program, error) {
	filename := map[string]string{
		"set":        "set.apex",
		"read":       "read.apex",
		"null_set":   "null_set.apex",
		"locked_set": "locked_set.apex",
	}[operation]
	if filename == "" {
		return ir.Program{}, fmt.Errorf("unknown feature parameter operation %q", operation)
	}
	source, err := os.ReadFile(filepath.Join(fixture, filename))
	if err != nil {
		return ir.Program{}, err
	}
	return vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
}

func featureParameterNativeCount(t *testing.T, metadata storage.MetadataRegistry) int {
	t.Helper()
	field := reflect.ValueOf(metadata).FieldByName("FeatureParameters")
	if !field.IsValid() {
		return 0
	}
	return field.Len()
}
