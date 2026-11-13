package lsp

import (
	"fmt"

	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestDefaultCompletionFiltersByPrefix(t *testing.T) {
	idx := typesys.Index{}
	for i := 0; i < 3000; i++ {
		idx.Types = append(idx.Types, typesys.TypeSymbol{Name: fmt.Sprintf("Latency%04d", i), Kind: apexast.DeclarationClass})
	}
	h := NewHandler(idx)
	uri := DocumentURI("file:///completion.cls")
	h.applyOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "Latency12"}})
	result := h.Completion(CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: 9}})
	if !result.IsIncomplete || len(result.Items) == 0 || len(result.Items) >= 2000 {
		t.Fatalf("unexpected completion: incomplete=%v count=%d", result.IsIncomplete, len(result.Items))
	}
	for _, item := range result.Items {
		if !completionSubsequence(item.Label, "Latency12") {
			t.Fatalf("unexpected item %q", item.Label)
		}
	}
}

func TestDefaultCompletionEmptyPrefixIsCapped(t *testing.T) {
	idx := typesys.Index{}
	for i := 0; i < 3000; i++ {
		idx.Types = append(idx.Types, typesys.TypeSymbol{Name: fmt.Sprintf("Type%04d", i)})
	}
	result := NewHandler(idx).Completion(CompletionParams{})
	if !result.IsIncomplete || len(result.Items) != 2000 {
		t.Fatalf("incomplete=%v count=%d", result.IsIncomplete, len(result.Items))
	}
}
