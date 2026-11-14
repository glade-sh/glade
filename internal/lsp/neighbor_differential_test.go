package lsp

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Compare these ordinary editor responses with the same test on the base.
func TestEditorUnchangedNeighbor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.cls")
	if err := os.WriteFile(path, []byte("class Sample { Sample value; }"), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(typesys.Index{Types: []typesys.TypeSymbol{{Name: "Sample", File: path}}})
	uri := uriFromPath(path)
	pos := Position{Line: 0, Character: 8}
	refs := h.References(ReferenceParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: pos, Context: ReferenceContext{IncludeDeclaration: true}})
	if len(refs) != 2 {
		t.Fatalf("reference count = %d, want 2", len(refs))
	}
	got := struct {
		References []Location
		Definition []Location
		Symbols    []SymbolInformation
		Completion CompletionList
	}{refs, h.Definition(DefinitionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: pos}), h.WorkspaceSymbols("Sample"), h.Completion(CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 0, Character: 14}})}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), dir, "PROJECT"))
	t.Logf("normalized editor responses SHA256 %x", sha256.Sum256(data))
}
