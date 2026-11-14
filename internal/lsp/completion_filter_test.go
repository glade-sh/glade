package lsp

import (
	"fmt"
	"strings"

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

func TestCompletionFilterEqualsCatalogSubset(t *testing.T) {
	catalog := NewHandler(benchmarkLSPIndex(20)).defaultCompletionCatalog(completionContextDefault)
	for _, prefix := range []string{"lat", "LS2", "zZ", ""} {
		var want []CompletionItem
		for _, item := range catalog {
			chars := []rune(strings.ToLower(prefix))
			for _, r := range strings.ToLower(item.Label) {
				if len(chars) > 0 && chars[0] == r {
					chars = chars[1:]
				}
			}
			if len(chars) == 0 {
				want = append(want, item)
				if len(want) == 2000 {
					break
				}
			}
		}
		got, _ := filterCompletionCatalog(catalog, prefix)
		if len(got) != len(want) {
			t.Fatalf("%q: got %d want %d", prefix, len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%q: item %d differs", prefix, i)
			}
		}
	}
}
