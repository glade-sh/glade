package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// TestEditorUnchangedNeighborOutput emits a stable response for comparison
// against origin/main for an unchanged reference and an open syntax error.
func TestEditorUnchangedNeighborOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Example.cls")
	text := "public class Example { Example item; }"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Index{Types: []typesys.TypeSymbol{{File: path}}})
	uri := uriFromPath(path)
	refs := h.References(ReferenceParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 0, Character: 15}, Context: ReferenceContext{IncludeDeclaration: true}})
	if len(refs) != 2 {
		t.Fatalf("references = %#v", refs)
	}
	for i := range refs {
		refs[i].URI = "file:///Example.cls"
	}
	notifications := h.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "public class Example {"}})
	if len(notifications) != 1 {
		t.Fatalf("diagnostics = %#v", notifications)
	}
	params := notifications[0].Params.(PublishDiagnosticsParams)
	if len(params.Diagnostics) == 0 {
		t.Fatal("missing parse diagnostics")
	}
	params.URI = "file:///Example.cls"
	result, err := json.Marshal(struct {
		References  []Location               `json:"references"`
		Diagnostics PublishDiagnosticsParams `json:"diagnostics"`
	}{refs, params})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("NEIGHBOR %s", result)
}
