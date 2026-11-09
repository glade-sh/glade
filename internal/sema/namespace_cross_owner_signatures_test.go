package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNamespaceCrossOwnerSignatures(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, member := range map[string]string{"parameter": "public void accept(host.ForeignShape value) {}", "return": "public host.ForeignShape getValue() { return null; }"} {
			t.Run(api+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				child := filepath.Join(root, "child")
				if err := os.MkdirAll(child, 0755); err != nil {
					t.Fatal(err)
				}
				foreign := filepath.Join(child, "ForeignShape.cls")
				consumer := filepath.Join(root, "SignatureOwner.cls")
				writeSemaFile(t, foreign, "public class ForeignShape {}")
				writeSemaFile(t, consumer, "public class SignatureOwner { "+member+" }")
				idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{foreign, consumer}}, schema.Schema{})
				for _, loaded := range []bool{false, true} {
					if loaded {
						idx.Dependencies = []typesys.DependencyInfo{{Namespace: "other", Status: "loaded", SourceRoot: child}}
					}
					result := Analyze(idx)
					found := false
					for _, d := range result.Diagnostics {
						if d.Severity == diagnostic.Error && d.NativeMessage == "Invalid type: host.ForeignShape" {
							found = true
						}
					}
					if !found {
						t.Errorf("loaded=%v missing native error: %+v", loaded, result.Diagnostics)
					}
				}
			})
		}
	}
}
