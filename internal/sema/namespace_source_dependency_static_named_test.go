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

func TestSourceDependencyNamedStaticNative(t *testing.T) {
	for _, name := range []string{"namespace_source_dependency_static_named", "namespace_source_dependency_static_named_qualified"} {
		t.Run(name, func(t *testing.T) { sourceDependencyNamedStaticNative(t, name) })
	}
}

func sourceDependencyNamedStaticNative(t *testing.T, name string) {
	t.Helper()
	root := filepath.Join("..", "..", "testdata", "conformance", name)
	f, err := os.Open(filepath.Join(root, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 7 {
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
			helper := filepath.Join(dep, "Nsv36HelperProbe.cls")
			writeSemaFile(t, helper, read("Nsv36HelperProbe"))
			owner := filepath.Join(dir, "Nsv36OwnerProbe.cls")
			writeSemaFile(t, owner, read("Nsv36OwnerProbe"))
			consumer := filepath.Join(dir, "Nsv36ConsumerProbe.cls")
			writeSemaFile(t, consumer, read("Nsv36ConsumerProbe"))
			provider := project.Project{Root: dep, Namespace: "synthetic", SourceAPIVersion: row[1], ApexFiles: []string{helper}}
			p := project.Project{Root: dir, Namespace: "synthetic", SourceAPIVersion: row[1], ApexFiles: []string{owner, consumer}, ManagedPackageDependencies: []project.ManagedPackageDependency{{Namespace: "synthetic", SourceRoot: dep, Status: "loaded", Project: &provider}}}
			index := typesys.Build(p, schema.Schema{})
			if index.HasErrors() {
				t.Fatalf("index: %+v", index.Diagnostics)
			}
			result := Analyze(index)
			matched := false
			var errors []string
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error {
					errors = append(errors, d.Message+" | "+d.NativeMessage)
					matched = matched || d.Message == row[4] || d.NativeMessage == row[4]
				}
			}
			if row[3] == "COMPILED" && len(errors) > 0 {
				t.Fatalf("native compiled; offline errors: %v", errors)
			}
			if row[3] == "COMPILE_ERROR" && (!matched || len(errors) != 1) {
				t.Fatalf("expected only native error %q; diagnostics: %+v", row[4], result.Diagnostics)
			}
			if row[0] == "missing_root" || row[0] == "missing_member" {
				// These are offline completeness controls, not claims about an
				// installed package: a partial index does not prove either miss,
				// and an opaque dependency does not prove its root absent.
				modes := []string{"partial"}
				if row[0] == "missing_root" {
					modes = append(modes, "opaque")
				}
				for _, mode := range modes {
					control := index
					if mode == "opaque" {
						control.Dependencies = append(append([]typesys.DependencyInfo(nil), index.Dependencies...), typesys.DependencyInfo{Namespace: "Nsv36AbsentProbe", Status: "missing"})
					} else {
						control.Subset = true
					}
					for _, d := range Analyze(control).Diagnostics {
						if d.Severity == diagnostic.Error && (d.Message == row[4] || d.NativeMessage == row[4]) {
							t.Errorf("%s index rejected an unproven receiver miss: %+v", mode, d)
						}
					}
				}
			}
		})
	}
}
