package lsp

import (
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
	// Captured by running this same test against origin/main at 0574a2848.
	const want = `{"References":[{"uri":"file://PROJECT/Sample.cls","range":{"start":{"line":0,"character":6},"end":{"line":0,"character":12}}},{"uri":"file://PROJECT/Sample.cls","range":{"start":{"line":0,"character":15},"end":{"line":0,"character":21}}}],"Definition":[{"uri":"file://PROJECT/Sample.cls","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}],"Symbols":[{"name":"Sample","kind":19,"location":{"uri":"file://PROJECT/Sample.cls","range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}}],"Completion":{"isIncomplete":false,"items":[{"label":"Sample","kind":7},{"label":"class","kind":14,"detail":"Apex keyword"},{"label":"else","kind":14,"detail":"Apex keyword"},{"label":"enum","kind":14,"detail":"Apex keyword"},{"label":"for","kind":14,"detail":"Apex keyword"},{"label":"global","kind":14,"detail":"Apex keyword"},{"label":"if","kind":14,"detail":"Apex keyword"},{"label":"interface","kind":14,"detail":"Apex keyword"},{"label":"new","kind":14,"detail":"Apex keyword"},{"label":"private","kind":14,"detail":"Apex keyword"},{"label":"protected","kind":14,"detail":"Apex keyword"},{"label":"public","kind":14,"detail":"Apex keyword"},{"label":"return","kind":14,"detail":"Apex keyword"},{"label":"static","kind":14,"detail":"Apex keyword"},{"label":"trigger","kind":14,"detail":"Apex keyword"},{"label":"void","kind":14,"detail":"Apex keyword"}]}}`
	if string(data) != want {
		t.Fatalf("normalized editor responses differ from base\ngot:  %s\nwant: %s", data, want)
	}
}
