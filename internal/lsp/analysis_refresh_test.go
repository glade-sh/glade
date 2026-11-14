package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Open-buffer diagnostics must follow semantic edits without saving to disk.
func TestDiagnosticsRefreshAfterSemanticEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "classes", "Draft.cls")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"67.0"}`), 0644); err != nil {
		t.Fatal(err)
	}
	initial := "public class Draft { public void run() { Integer x = 1; } }"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Build(p, schema.Schema{}))
	uri := uriFromPath(path)
	opened := h.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Version: 1, Text: initial}})
	if n := diagnosticsCount(t, opened); n != 0 {
		t.Fatalf("initial diagnostics: %v", opened)
	}
	edited := "public class Draft { public void run() { Integer x = 'wrong'; } }"
	changed, err := h.DidChange(DidChangeTextDocumentParams{TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []TextDocumentContentChangeEvent{{Text: edited}}})
	if err != nil {
		t.Fatal(err)
	}
	if n := diagnosticsCount(t, changed); n == 0 {
		t.Fatalf("semantic edit produced no diagnostics: %v", changed)
	}
}
