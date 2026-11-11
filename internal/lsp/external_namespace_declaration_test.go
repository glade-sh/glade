package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Ruling 49(1): the declared twin of TestUndeclaredNamespaceTypesLSPRoutes.
// With glade.yml project.externalNamespaces: [bar, fooNs] the same documents
// publish no error: a declared external namespace stays open.
func TestExternalNamespaceDeclarationLSPRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/undeclared_namespace_types/project/force-app/main/default/classes"
	files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 5 {
		t.Fatalf("fixture classes=%d", len(files))
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, declared := range []string{"", "bar, fooNs"} {
			t.Run(api+"/declared="+declared, func(t *testing.T) {
				root := t.TempDir()
				classes := filepath.Join(root, "force-app", "main", "default", "classes")
				if err := os.MkdirAll(classes, 0o755); err != nil {
					t.Fatal(err)
				}
				writeReceiverMemberFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				if declared != "" {
					writeReceiverMemberFile(t, filepath.Join(root, "glade.yml"), "project:\n  externalNamespaces: ["+declared+"]\n")
				}
				var paths []string
				for _, file := range files {
					data, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(classes, filepath.Base(file))
					writeReceiverMemberFile(t, path, string(data))
					paths = append(paths, path)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				h := NewHandler(index)
				defer h.parser.Close()
				errors := 0
				for _, path := range paths {
					for _, d := range h.diagnosticsForDocument(uriFromPath(path)) {
						if d.Severity == diagnostic.Error {
							errors++
							if declared != "" {
								t.Errorf("%s: declared namespace published %s %q", filepath.Base(path), d.Code, d.NativeMessage)
							}
						}
					}
				}
				if declared == "" && errors == 0 {
					t.Fatal("undeclared namespaces accepted")
				}
			})
		}
	}
}
