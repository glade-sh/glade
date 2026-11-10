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

func TestSameNamespaceDependencyForeignStaticNative(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "conformance", "namespace_foreign_static_subscriber")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	provider, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", "namespace_subscriber_members", "provider", "classes", "V035Provider.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			tmp := t.TempDir()
			foreignRoot := filepath.Join(tmp, "foreign")
			dep := filepath.Join(tmp, "provider")
			for _, dir := range []string{foreignRoot, dep} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			source, err := os.ReadFile(filepath.Join(root, row[6]))
			if err != nil {
				t.Fatal(err)
			}
			sourceText := strings.ReplaceAll(string(source), "{{NAMESPACE}}", "host")
			foreign := filepath.Join(foreignRoot, "ForeignCaller.cls")
			writeSemaFile(t, foreign, "public class ForeignCaller { public static void check() { "+sourceText+" } }")
			providerFile := filepath.Join(dep, "V035Provider.cls")
			writeSemaFile(t, providerFile, string(provider))
			depProject := project.Project{Root: dep, Namespace: "host", SourceAPIVersion: row[1], ApexFiles: []string{providerFile}}
			p := project.Project{Root: tmp, Namespace: "host", SourceAPIVersion: row[1], SourceNamespaces: []string{"foreign"}, SourceNamespaceRoots: map[string]string{foreignRoot: "foreign"}, ApexFiles: []string{foreign}, ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "host", SourceRoot: dep, Status: "loaded", Project: &depProject}}}
			index := typesys.Build(p, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index errors: %+v", index.Diagnostics)
			}
			want := strings.ReplaceAll(row[5], "{{NAMESPACE}}", "host")
			matched := false
			var errors, relevant []string
			for _, d := range Analyze(index).Diagnostics {
				if filepath.Clean(d.File) != filepath.Clean(foreign) {
					continue
				}
				if strings.Contains(d.Message, "V035Provider") || strings.Contains(d.NativeMessage, "V035Provider") {
					relevant = append(relevant, d.Message+" | "+d.NativeMessage)
				}
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					matched = matched || d.Message == want || d.NativeMessage == want
				}
			}
			if row[3] == "COMPILED" && (len(errors) > 0 || len(relevant) > 0) {
				t.Fatalf("native compiles; errors=%v relevant=%v", errors, relevant)
			}
			if row[3] == "COMPILE_ERROR" && !matched {
				t.Fatalf("native diagnostic %q not found: errors=%v relevant=%v", want, errors, relevant)
			}
		})
	}
}
