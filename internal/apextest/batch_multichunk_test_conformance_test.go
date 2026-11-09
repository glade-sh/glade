package apextest

import (
	"encoding/csv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

// Native probe 2026-11-06: a multi-chunk executeBatch in a test method or in
// @TestSetup, with a QueryLocator or Iterable start, raises an uncatchable
// System.UnexpectedException.
func TestBatchMultiChunkTestNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "batch_multichunk_test")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("invalid native observations: %v", err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				if strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, "sfdx-project.json") {
					body = []byte(strings.ReplaceAll(string(body), "62.0", api))
				}
				writeFile(t, filepath.Join(root, rel), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			seen := 0
			for _, row := range rows[1:] {
				if len(row) != 5 {
					t.Fatal("malformed native row")
				}
				if row[0] != api {
					continue
				}
				seen++
				t.Run(row[1]+"/"+row[2], func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{row[1]}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
						t.Fatalf("expected selected method: %+v", run.Summary())
					}
					problem := run.Suites[0].Cases[0].Problem
					if problem == nil {
						t.Fatal("missing problem")
					}
					if row[3] == "UNCAUGHT" {
						if problem.Type != "System.UnexpectedException" || problem.Message != row[4] {
							t.Fatalf("native=%q; Glade=%+v", row[4], problem)
						}
						return
					}
					if !strings.HasSuffix(problem.Type, "AssertException") || !strings.HasSuffix(problem.Message, "CAP|"+row[4]) {
						t.Fatalf("native=%q; Glade=%+v", row[4], problem)
					}
				})
			}
			if seen != 6 {
				t.Fatal("incomplete source API observations")
			}
		})
	}
}
