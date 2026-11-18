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

func TestNamespaceSweepSameNamespaceDependencyNative(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "conformance", "namespace_sweep_same_namespace")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("native rows: %d: %v", len(rows), err)
	}
	provider, err := os.ReadFile(filepath.Join(root, "V036SameOwner.cls"))
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
			writeSemaFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"synthetic-provider"}]}]}`)
			writeSemaFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  dependencyPaths: [../provider]\n")
			writeSemaFile(t, filepath.Join(dep, "sfdx-project.json"), `{"namespace":"v036ns","sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true,"package":"synthetic-provider"}]}`)
			writeSemaFile(t, filepath.Join(dep, "force-app", "V036SameOwner.cls"), string(provider))
			source, err := os.ReadFile(filepath.Join(root, row[5]))
			if err != nil {
				t.Fatal(err)
			}
			writeSemaFile(t, filepath.Join(consumer, "force-app", "V036SameConsumer.cls"), string(source))
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
			var errors, ownerDiagnostics []string
			matched := false
			for _, d := range Analyze(index).Diagnostics {
				if strings.Contains(d.Message, "V036SameOwner") || strings.Contains(d.NativeMessage, "V036SameOwner") {
					ownerDiagnostics = append(ownerDiagnostics, string(d.Severity)+": "+d.Message+" | "+d.NativeMessage)
				}
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					matched = matched || d.Message == row[4] || d.NativeMessage == row[4]
				}
			}
			if row[3] == "COMPILED" && (len(errors) > 0 || len(ownerDiagnostics) > 0) {
				t.Fatalf("native compiles; offline diagnostics: %s; owner: %s; dependencies: %+v; types: %+v", strings.Join(errors, "; "), strings.Join(ownerDiagnostics, "; "), index.Dependencies, index.Types)
			}
			if row[3] == "COMPILE_ERROR" && !matched {
				t.Fatalf("native diagnostic %q not found: %v; dependencies: %+v; types: %+v", row[4], errors, index.Dependencies, index.Types)
			}
		})
	}
}
