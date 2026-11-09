package apextest

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Salesforce rows establish typed output and null assignment for declared
// invocable output fields at the floor and ceiling API versions.
func TestFlowActionOutputCapture(t *testing.T) {
	fixture := "testdata/conformance/flow_action_output_capture"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixture, "api"+strings.TrimSuffix(api, ".0"), "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "OutputProbeTest.single\tPass") {
				t.Fatalf("native row missing: %s", raw)
			}
			root := t.TempDir()
			src := filepath.Join(fixture, "project")
			err = filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				text := string(body)
				if strings.HasSuffix(rel, "-meta.xml") || strings.HasSuffix(rel, "sfdx-project.json") {
					text = strings.ReplaceAll(text, "62.0", api)
				}
				writeFile(t, filepath.Join(root, rel), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			report := RunContext(context.Background(), index, Options{NoDiskCache: true, Parallelism: 1})
			count := 0
			for _, suite := range report.Suites {
				for _, c := range suite.Cases {
					count++
					if string(c.Status) != "pass" {
						t.Fatalf("%s.%s: %s %+v", suite.Name, c.Name, c.Status, c.Problem)
					}
				}
			}
			if count != 2 {
				t.Fatalf("cases=%d", count)
			}
		})
	}
}
