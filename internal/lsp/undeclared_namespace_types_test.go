package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Synthetic: ../../testdata/conformance/undeclared_namespace_types/cases.md.
// Glade a recorded revision published no diagnostics for these documents.
func TestUndeclaredNamespaceTypesLSPRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/undeclared_namespace_types/project/force-app/main/default/classes"
	files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 5 {
		t.Fatalf("fixture classes=%d", len(files))
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "fooNs"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for _, file := range files {
					data, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, filepath.Base(file))
					writeReceiverMemberFile(t, path, string(data))
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				h := NewHandler(index)
				defer h.parser.Close()
				for _, path := range paths {
					if ds := h.diagnosticsForDocument(uriFromPath(path)); len(ds) != 0 {
						t.Fatalf("%s: %+v", filepath.Base(path), ds)
					}
				}
			})
		}
	}
}

// System's inventory is closed; native invalid_system rows reject this.
func TestUndeclaredNamespaceLSPKeepsSystemAbsence(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "SystemAbsent.cls")
			writeReceiverMemberFile(t, path, "public class SystemAbsent { public Object make() { return new System.NotAType(); } }")
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			h := NewHandler(index)
			defer h.parser.Close()
			for _, d := range h.diagnosticsForDocument(uriFromPath(path)) {
				if d.NativeMessage == "Invalid type: System.NotAType" {
					return
				}
			}
			t.Fatal("native System.NotAType rejection missing")
		})
	}
}
