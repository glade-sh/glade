package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A source-backed dependency must not borrow a local project's suffix. Native
// subscriber controls and rejection: testdata/namespace_cross_owner/org.tsv.
func TestNamespaceCrossOwnerLoadedDependency(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			child := filepath.Join(root, "child")
			if err := os.MkdirAll(child, 0755); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(root, "LocalShape.cls")
			foreign := filepath.Join(child, "ForeignShape.cls")
			test := filepath.Join(root, "CrossOwnerTest.cls")
			writeSemaFile(t, local, "public class LocalShape {}")
			writeSemaFile(t, foreign, "public class ForeignShape {}")
			writeSemaFile(t, test, "@IsTest private class CrossOwnerTest { @IsTest static void check() { other.LocalShape bad; } }")
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, test}}, schema.Schema{})
			idx.Dependencies = []typesys.DependencyInfo{{Namespace: "other", Status: "loaded", SourceRoot: child}}
			if result := Analyze(idx); !result.HasErrors() {
				t.Errorf("named test accepted foreign suffix: %+v", result.Diagnostics)
			}
			for _, source := range []string{"other.LocalShape bad;", "host.ForeignShape bad;"} {
				if result := AnalyzeAnonymous(idx, source, api); !result.HasErrors() {
					t.Errorf("accepted %s", source)
				}
			}
		})
	}
}
