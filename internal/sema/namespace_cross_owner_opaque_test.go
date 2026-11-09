package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// An explicitly declared opaque dependency stays open even when source with
// the same namespace is loaded. Exercise both cold and cached anonymous setup.
func TestNamespaceCrossOwnerOpaqueAndWarm(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(root, "LocalShape.cls")
	foreign := filepath.Join(child, "ForeignShape.cls")
	writeSemaFile(t, local, "public class LocalShape {}")
	writeSemaFile(t, foreign, "public class ForeignShape {}")
	idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: "67.0", SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign}}, schema.Schema{})
	idx.Dependencies = []typesys.DependencyInfo{{Namespace: "other", Status: "missing"}}
	for i := 0; i < 2; i++ {
		if got := AnalyzeAnonymous(idx, "other.LocalShape value;", "67.0"); got.HasErrors() {
			t.Fatalf("call %d: opaque namespace rejected: %+v", i, got.Diagnostics)
		}
	}
}
