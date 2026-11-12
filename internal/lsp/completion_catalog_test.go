package lsp

import "testing"

func TestDefaultCompletionCatalogIsBuiltOncePerOrdering(t *testing.T) {
	handler := NewHandler(benchmarkLSPIndex(5))
	first := handler.Completion(CompletionParams{})
	second := handler.Completion(CompletionParams{})
	if len(first.Items) == 0 || len(first.Items) != len(second.Items) {
		t.Fatalf("items = %d, then %d", len(first.Items), len(second.Items))
	}
	if &first.Items[0] != &second.Items[0] {
		t.Fatal("the default catalog was rebuilt for the second request")
	}
	for i := 1; i < len(first.Items); i++ {
		if first.Items[i-1].Label > first.Items[i].Label {
			t.Fatalf("catalog not sorted at %d: %q after %q", i, first.Items[i].Label, first.Items[i-1].Label)
		}
	}
	// A SOQL field context has its own ordering and must not reuse the default.
	soql := handler.defaultCompletionCatalog(completionContextSOQLSelect)
	if len(soql) != len(first.Items) || soql[0].Kind != completionItemKindField {
		t.Fatalf("soql catalog first item = %#v", soql[0])
	}
}
