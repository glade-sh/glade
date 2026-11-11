package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNamespaceSweepShapesNative(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "conformance", "namespace_sweep_shapes")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows: %d: %v", len(rows), err)
	}
	provider, err := os.ReadFile(filepath.Join(root, "V036ReviewOuter.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, "V036ReviewOuter.cls")
			writeSemaFile(t, file, string(provider))
			index := typesys.Build(project.Project{Root: dir, SourceAPIVersion: row[1], ApexFiles: []string{file}}, schema.Schema{})
			index.Dependencies = []typesys.DependencyInfo{{Namespace: "unavailable-package", Status: "missing"}}
			source, err := os.ReadFile(filepath.Join(root, row[6]))
			if err != nil {
				t.Fatal(err)
			}
			var errors []string
			matched := false
			for _, d := range AnalyzeAnonymous(index, string(source), row[1]).Diagnostics {
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					matched = matched || d.Message == row[5] || d.NativeMessage == row[5]
				}
			}
			if row[3] == "COMPILED" && len(errors) > 0 {
				t.Fatalf("native compiles; offline errors: %s", strings.Join(errors, "; "))
			}
			if row[3] == "COMPILE_ERROR" && !matched {
				t.Fatalf("native diagnostic %q not found: %v", row[5], errors)
			}
		})
	}
}
