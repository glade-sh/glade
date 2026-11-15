package lsp

import (
	"encoding/json"
	"testing"
)

// TestClosedDocumentNeighbor emits the response bytes for comparison with main.
func TestClosedDocumentNeighbor(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	uri := uriFromPath(idx.Types[0].File)
	result := []any{
		h.DocumentSymbols(uri),
		h.Hover(HoverParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Line: 1, Character: 4}}),
		h.SemanticTokensFull(SemanticTokensParams{TextDocument: TextDocumentIdentifier{URI: uri}}),
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("NEIGHBOR %s", data)
}
