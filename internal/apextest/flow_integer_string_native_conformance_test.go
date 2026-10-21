package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowIntegerStringAssignmentNativeConformance(t *testing.T) {
	const directory = "testdata/conformance/flow_integer_string_assignment"
	raw, err := os.ReadFile(filepath.Join(directory, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(rows) != 7 || rows[0] != "source_api\troute\tobservation" {
		t.Fatal("incomplete native observations")
	}
	anonymous, err := os.ReadFile(filepath.Join(directory, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, line := range rows[1:] {
		columns := strings.SplitN(line, "\t", 3)
		if len(columns) != 3 {
			t.Fatal("invalid native observation")
		}
		api, route := columns[0], columns[1]
		if (api != "62.0" && api != "67.0" && api != "68.0") || (route != "anonymous" && route != "istest") || seen[api+route] {
			t.Fatal("unknown or duplicate native observation")
		}
		seen[api+route] = true
		t.Run(api+"/"+route, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(directory, "project"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(filepath.Join(directory, "project"), path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var message string
			if route == "anonymous" {
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Debug) != 1 {
					t.Fatalf("missing native debug transport: %v", result.Debug)
				}
				message = result.Debug[0]
			} else {
				run := Run(index, Options{SelectedClasses: []string{"ScalarObservationTest"}, SelectedMethod: "numberText", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("unexpected selection: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing native assertion transport: %+v", result)
				}
				message = result.Problem.Message
			}
			_, payload, found := strings.Cut(message, "OBSERVED|numberText|")
			var got, want map[string]any
			if !found || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(columns[2]), &want) != nil {
				t.Fatalf("invalid observation: %s", message)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("native=%s; Glade=%s", columns[2], payload)
			}
		})
	}
}
