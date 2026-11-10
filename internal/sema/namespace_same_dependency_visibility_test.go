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

func TestSameNamespaceDependencyVisibilityNative(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "namespace_same_dependency_visibility")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("native rows: %d: %v", len(rows), err)
	}
	provider, err := os.ReadFile(filepath.Join(root, "V036AccessOwner.cls"))
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
			writeSemaFile(t, filepath.Join(dep, "force-app", "V036AccessOwner.cls"), string(provider))
			source, err := os.ReadFile(filepath.Join(root, row[5]))
			if err != nil {
				t.Fatal(err)
			}
			sourceText := strings.ReplaceAll(string(source), "{{NAMESPACE}}", "v036ns")
			writeSemaFile(t, filepath.Join(consumer, "force-app", "V036AccessConsumer.cls"), sourceText)
			p, err := project.Load(consumer)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, schema.Schema{})
			loaded := false
			for _, d := range index.Dependencies {
				loaded = loaded || d.Status == "loaded"
			}
			if !loaded {
				t.Fatal("provider dependency not loaded")
			}
			want := strings.ReplaceAll(row[4], "{{NAMESPACE}}", "v036ns")
			var errors, relevant, all []string
			matched := false
			for _, d := range Analyze(index).Diagnostics {
				all = append(all, string(d.Severity)+" "+d.Code+": "+d.Message+" | "+d.NativeMessage)
				if strings.Contains(d.Message, "V036AccessOwner") || strings.Contains(d.NativeMessage, "V036AccessOwner") {
					relevant = append(relevant, d.Message+" | "+d.NativeMessage)
				}
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					matched = matched || d.Message == want || d.NativeMessage == want
				}
			}
			if row[3] == "COMPILED" && (len(errors) > 0 || len(relevant) > 0) {
				t.Fatalf("native compiles; offline errors %v; relevant %v", errors, relevant)
			}
			if row[3] == "COMPILE_ERROR" && !matched {
				t.Fatalf("native diagnostic %q not found: errors=%v all=%v", want, errors, all)
			}
		})
	}
}
