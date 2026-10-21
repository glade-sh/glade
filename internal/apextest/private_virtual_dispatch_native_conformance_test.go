package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestPrivateMethodDeclaringVersionNativeConformance(t *testing.T) {
	const fixtures = "testdata/conformance/private_virtual_dispatch"
	f, err := os.Open(filepath.Join(fixtures, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 || strings.Join(rows[0], ",") != "api,class,method,outcome" {
		t.Fatalf("incomplete native matrix: rows=%d err=%v", len(rows), err)
	}
	var classes []string
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if len(row) != 4 || !slices.Contains([]string{"52.0", "60.0", "61.0", "62.0", "67.0"}, row[0]) || row[3] != "Pass" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := row[1] + "." + row[2]
		if seen[key] {
			t.Fatal("duplicate native row: " + key)
		}
		seen[key] = true
		if !slices.Contains(classes, row[1]) {
			classes = append(classes, row[1])
		}
	}
	if len(classes) != 7 {
		t.Fatalf("native declaring/caller version controls missing: %v", classes)
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixtures, "project"))); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("native sources compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	run := Run(index, Options{SelectedClasses: classes, NoDiskCache: true, Parallelism: 1})
	observed := make(map[string]testreport.Case)
	for _, suite := range run.Suites {
		for _, result := range suite.Cases {
			key := result.ClassName + "." + result.MethodName
			if _, duplicate := observed[key]; duplicate || !seen[key] {
				t.Fatalf("unexpected or duplicate local observation: %s", key)
			}
			observed[key] = result
		}
	}
	for _, row := range rows[1:] {
		key := row[1] + "." + row[2]
		t.Run(row[0]+"/"+key, func(t *testing.T) {
			result, ok := observed[key]
			if !ok || result.Status != testreport.StatusPass {
				t.Fatalf("native=%s; Glade=%s problem=%+v", row[3], result.Status, result.Problem)
			}
		})
	}
}
