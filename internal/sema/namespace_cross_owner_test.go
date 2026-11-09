package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native: testdata/namespace_cross_owner/org.tsv. Each synthetic namespace
// contains only its own source class. Test both directions and both routes.
func TestNamespaceCrossOwnerLocalDeclarations(t *testing.T) {
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
			body := "host.LocalShape a;\nother.ForeignShape b;\nhost.ForeignShape c;\nother.LocalShape d;"
			writeSemaFile(t, test, "@IsTest private class CrossOwnerTest { @IsTest static void check() { "+body+" } }")
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, test}}, schema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			want := []string{"host.ForeignShape", "other.LocalShape"}
			assertOnlyLocalNamespaceRejections(t, "anonymous", AnalyzeAnonymous(idx, body, api).Diagnostics, want)
			assertOnlyLocalNamespaceRejections(t, "istest", Analyze(idx).Diagnostics, want)
		})
	}
}
