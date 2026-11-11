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
)

// The native inventory records profile and license relationships in a plain
// Developer org at both supported Apex endpoints.
func TestProfileInventoryShapeNative(t *testing.T) {
	capture := setupRenderingCaptures[0]
	rows := readSetupRenderingCaptures(t, capture)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join("testdata", "conformance", capture.family, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
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
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{capture.testClass}, SelectedMethod: "inventory", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing observation: %+v", run.Summary())
			}
			_, gotJSON, found := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "CAPTURE|")
			if !found {
				t.Fatalf("missing capture marker: %s", run.Suites[0].Cases[0].Problem.Message)
			}
			var got, want any
			if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				if row[0] == api && row[1] == "plain" && row[2] == "isTest" && row[3] == "inventory" {
					if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if want == nil || !reflect.DeepEqual(got, want) {
				t.Errorf("native inventory=%v; Glade=%v", want, got)
			}
		})
	}
}
