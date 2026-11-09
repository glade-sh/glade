package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNamespaceCrossOwnerSameName(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			child := filepath.Join(root, "child")
			if err := os.MkdirAll(child, 0755); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(root, "SharedShape.cls")
			foreign := filepath.Join(child, "SharedShape.cls")
			test := filepath.Join(root, "TwinTest.cls")
			writeSemaFile(t, local, "public class SharedShape {}")
			writeSemaFile(t, foreign, "public class SharedShape {}")
			body := "host.SharedShape first;\nother.SharedShape second;"
			writeSemaFile(t, test, "@IsTest private class TwinTest { @IsTest static void check() { "+body+" } }")
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, test}}, schema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			for route, result := range map[string]Result{"anonymous": AnalyzeAnonymous(idx, body, api), "istest": Analyze(idx)} {
				if result.HasErrors() {
					t.Errorf("%s rejected distinct qualified types: %+v", route, result.Diagnostics)
				}
			}
		})
	}
}
