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

// Native probe 2026-11-06: System.scheduleBatch from a batch finish is rejected
// only for a batch class at API 25 or older; start and execute always reject.
func TestBatchFinishScheduleAPINativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "batch_finish_schedule_api")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 25 {
		t.Fatalf("invalid native observations: %v", err)
	}
	root := t.TempDir()
	project := filepath.Join(dir, "project")
	err = filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
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
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("malformed native row")
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			run := Run(index, Options{SelectedClasses: []string{row[0]}, SelectedMethod: row[1], NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("expected selected method: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			want := "CAP|" + row[1] + "|" + row[2] + "|" + row[3]
			if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") || !strings.HasSuffix(problem.Message, want) {
				t.Fatalf("native=%q; Glade=%+v", want, problem)
			}
		})
	}
}
