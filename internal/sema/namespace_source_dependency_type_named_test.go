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

func TestSourceDependencyNamedTypeNative(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "namespace_source_dependency_type_named")
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 13 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			dir := t.TempDir()
			dep := filepath.Join(dir, "dep")
			if err := os.MkdirAll(dep, 0755); err != nil {
				t.Fatal(err)
			}
			caseDir := filepath.Join(root, "cases", row[1]+"-"+row[0])
			read := func(name string) string {
				b, e := os.ReadFile(filepath.Join(caseDir, name+".cls"))
				if e != nil {
					t.Fatal(e)
				}
				return strings.ReplaceAll(string(b), "{{NAMESPACE}}", "synthetic")
			}
			helper := filepath.Join(dep, "Nsv36TypeHelper.cls")
			writeSemaFile(t, helper, read("Nsv36TypeHelper"))
			owner := filepath.Join(dir, "Nsv36TypeOwner.cls")
			writeSemaFile(t, owner, read("Nsv36TypeOwner"))
			consumer := filepath.Join(dir, "Nsv36TypeConsumer.cls")
			writeSemaFile(t, consumer, read("Nsv36TypeConsumer"))
			provider := project.Project{Root: dep, Namespace: "synthetic", SourceAPIVersion: row[1], ApexFiles: []string{helper}}
			p := project.Project{Root: dir, Namespace: "synthetic", SourceAPIVersion: row[1], ApexFiles: []string{owner, consumer}, ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "synthetic", SourceRoot: dep, Status: "loaded", Project: &provider}}}
			index := typesys.Build(p, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			result := Analyze(index)
			want, _, _ := strings.Cut(row[4], "; ")
			matched := false
			var errors []string
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					matched = matched || d.Message == want || d.NativeMessage == want
				}
			}
			if row[3] == "COMPILED" && len(errors) > 0 {
				t.Fatalf("native compiled; offline errors=%v", errors)
			}
			if row[3] == "COMPILE_ERROR" && !matched {
				t.Fatalf("native error %q not found; diagnostics=%+v", want, result.Diagnostics)
			}
		})
	}
}
