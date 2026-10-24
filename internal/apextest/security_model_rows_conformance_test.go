package apextest

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestSecurityModelRowsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/security_model_rows"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.WalkDir(filepath.Join(fixture, "project"), func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(filepath.Join(fixture, "project"), path)
				if err != nil {
					return err
				}
				source, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := strings.ReplaceAll(strings.ReplaceAll(string(source), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"), `"sourceApiVersion": "62.0"`, `"sourceApiVersion": "`+api+`"`)
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, route := range []string{"istest", "anonymous"} {
				t.Run(route, func(t *testing.T) {
					path := filepath.Join(fixture, api, "org.tsv")
					if route == "anonymous" {
						path = filepath.Join(fixture, api, "anonymous", "org.tsv")
					}
					raw, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					reader := csv.NewReader(strings.NewReader(string(raw)))
					reader.Comma = '\t'
					rows, err := reader.ReadAll()
					if err != nil {
						t.Fatal(err)
					}
					if len(rows) != 27 || strings.Join(rows[0], "|") != "case|value" {
						t.Fatalf("incomplete native rows: %d", len(rows))
					}
					program, err := vm.CompileAnonymousWithOptions(`System.assert(false,'SECURITY_ROWS|'+JSON.serialize(SecurityModelProbe.rows()));`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := runner.newMachine()
					if route == "istest" {
						machine.EnableTestContext()
					}
					_, err = machine.Execute(program)
					var runtimeErr *vm.RuntimeError
					if !errors.As(err, &runtimeErr) {
						t.Fatalf("missing observation: %v", err)
					}
					_, payload, found := strings.Cut(runtimeErr.ExceptionMessage(), "SECURITY_ROWS|")
					if !found {
						t.Fatalf("observation failed: %v", err)
					}
					observed := map[string]string{}
					if err = json.Unmarshal([]byte(payload), &observed); err != nil {
						t.Fatal(err)
					}
					if len(observed) != len(rows)-1 {
						t.Fatalf("observed %d rows, want %d", len(observed), len(rows)-1)
					}
					for _, row := range rows[1:] {
						t.Run(row[0], func(t *testing.T) {
							if got, ok := observed[row[0]]; !ok || got != row[1] {
								t.Errorf("native=%q; glade=%q", row[1], got)
							}
						})
					}
				})
			}
		})
	}
}
