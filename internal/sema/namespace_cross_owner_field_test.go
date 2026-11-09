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

func TestNamespaceCrossOwnerFields(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			child := filepath.Join(root, "child")
			if err := os.MkdirAll(child, 0755); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(root, "LocalShape.cls")
			foreign := filepath.Join(child, "ForeignShape.cls")
			consumer := filepath.Join(root, "FieldHolder.cls")
			writeSemaFile(t, local, "public class LocalShape {}")
			writeSemaFile(t, foreign, "public class ForeignShape {}")
			writeSemaFile(t, consumer, "public class FieldHolder { public host.ForeignShape bad; public other.ForeignShape good; }")
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{local, foreign, consumer}}, schema.Schema{})
			if idx.HasErrors() {
				t.Fatal(idx.Diagnostics)
			}
			found := false
			for _, d := range Analyze(idx).Diagnostics {
				if d.Severity != diagnostic.Error {
					continue
				}
				if d.NativeMessage == "Invalid type: host.ForeignShape" {
					found = true
				} else {
					t.Errorf("unexpected diagnostic: %+v", d)
				}
			}
			if !found {
				t.Error("host.ForeignShape field accepted from other namespace")
			}
		})
	}
}
