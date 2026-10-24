package lsp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDerivedWorkNativeNamedLSPRoutes(t *testing.T) {
	for _, family := range []string{"derived_collection_traversal", "finalizer_deferred_work", "metadata_master_label"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(family+"/"+api, func(t *testing.T) {
				fixture := filepath.Join("..", "apextest", "testdata", "conformance", family)
				if _, err := os.Stat(filepath.Join(fixture, "org.tsv")); err != nil {
					t.Fatal(err)
				}
				source := filepath.Join(fixture, "project")
				root := t.TempDir()
				err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					name, err := filepath.Rel(source, path)
					if err != nil {
						return err
					}
					target := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
						return err
					}
					return os.WriteFile(target, []byte(strings.ReplaceAll(string(body), "62.0", api)), 0644)
				})
				if err != nil {
					t.Fatal(err)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, s)
				handler := NewHandler(index)
				defer handler.parser.Close()
				classes, err := filepath.Glob(filepath.Join(root, "force-app/main/default/classes/*.cls"))
				if err != nil {
					t.Fatal(err)
				}
				if len(classes) != 2 {
					t.Fatal("missing captured classes")
				}
				for _, path := range classes {
					body, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					notes := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(path), Version: 1, Text: string(body)}})
					for _, note := range notes {
						if payload, ok := note.Params.(PublishDiagnosticsParams); ok {
							for _, diagnostic := range payload.Diagnostics {
								if diagnostic.Severity == diagnosticSeverityError {
									t.Errorf("native accepted %s; LSP=%v", filepath.Base(path), diagnostic)
								}
							}
						}
					}
				}
			})
		}
	}
}
