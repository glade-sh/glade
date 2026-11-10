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

func TestNamespaceSweepInstalledShapesNative(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "namespace_sweep_shapes_installed")
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
	provider, err := os.ReadFile(filepath.Join(root, "V036EnumProvider.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			tmp := t.TempDir()
			consumer := filepath.Join(tmp, "consumer")
			dep := filepath.Join(tmp, "provider")
			for _, dir := range []string{filepath.Join(consumer, "force-app"), filepath.Join(dep, "force-app")} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			writeSemaFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"synthetic-provider"}]}]}`)
			writeSemaFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [../provider]\n")
			writeSemaFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true,"package":"synthetic-provider"}]}`)
			writeSemaFile(t, filepath.Join(dep, "force-app", "V036EnumProvider.cls"), string(provider))
			p, err := project.Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			loaded := false
			for _, d := range index.Dependencies {
				if d.Status == "loaded" {
					loaded = true
				}
			}
			if !loaded {
				t.Fatal("provider dependency not loaded")
			}
			code, err := os.ReadFile(filepath.Join(root, row[6]))
			if err != nil {
				t.Fatal(err)
			}
			var errors []string
			matched := false
			for _, d := range AnalyzeAnonymous(index, string(code), row[1]).Diagnostics {
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
			if row[0] == "missing_member_static" {
				index.Subset = true
				for _, d := range AnalyzeAnonymous(index, string(code), row[1]).Diagnostics {
					if d.Severity == diagnostic.Error {
						t.Fatalf("partial package inventory proved absence: %+v", d)
					}
				}
			}
		})
	}
}
