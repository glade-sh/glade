package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestUnlockedSubscriberDeclarationPositions(t *testing.T) {
	const root = "../apextest/testdata/conformance/namespace_unlocked_subscriber/installed_positions"
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	f.Close()
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		if len(row) != 6 {
			t.Fatalf("malformed native row: %q", row)
		}
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(row[5]))
			code, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			projectRoot := t.TempDir()
			provider := unlockedCapturedProvider(t, projectRoot)
			path = filepath.Join(projectRoot, filepath.Base(path))
			writeSemaFile(t, path, string(code))
			index := typesys.Build(project.Project{Root: projectRoot, SourceAPIVersion: row[1], ApexFiles: []string{provider, path}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index: %v", index.Diagnostics)
			}
			index.Dependencies = []typesys.DependencyInfo{{Namespace: "v036-unlocked", Status: "missing"}}
			var errors []string
			matched := false
			for _, d := range Analyze(index).Diagnostics {
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					if d.Message == row[4] || d.NativeMessage == row[4] {
						matched = true
					}
				}
			}
			if row[3] == "COMPILED" && len(errors) != 0 {
				t.Fatalf("native compiles, offline rejects: %v", errors)
			}
			if row[3] == "COMPILE_ERROR" && !matched {
				t.Fatalf("native rejection %q missing: %v", row[4], errors)
			}
		})
	}
}
