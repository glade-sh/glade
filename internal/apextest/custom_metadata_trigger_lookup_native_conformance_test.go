package apextest

import (
	"encoding/csv"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE: testdata/conformance/custom_metadata_trigger_lookup/org.tsv:2-5.
func TestCustomMetadataTriggerLookupNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/custom_metadata_trigger_lookup"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		id, api, route := row[0], row[1], row[2]
		key := api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate row %s", key)
		}
		seen[key] = true
		if id == "trigger" {
			if route != "anonymous" || row[3] != "True" || row[4] != "True" || row[5] != "TRIGGER=pass" {
				t.Fatalf("unexpected row: %q", row)
			}
		} else if id == "trigger_test" {
			if route != "istest" || row[3] != "true" || row[4] != "Passed" || row[5] != "run=Pass" {
				t.Fatalf("unexpected row: %q", row)
			}
		} else {
			t.Fatalf("unexpected case: %q", row)
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			src := filepath.Join(fixture, "force-app")
			err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				b, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, "force-app", rel), strings.ReplaceAll(string(b), "__API__", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{"MetadataTriggerTest"}, NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("result: %+v; %s", s, firstRunProblem(run))
				}
				return
			}
			body, err := os.ReadFile(filepath.Join(fixture, "trigger.apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("missing native rows: %v", seen)
	}
}
