package lsp

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestRequestUsesOneGeneration(t *testing.T) {
	idx := sampleIndex(t)
	h := NewHandler(idx)
	request := h.snapshot()
	changed := idx
	changed.Types = append([]typesys.TypeSymbol(nil), idx.Types...)
	changed.Types[0].Name = "Replacement"
	h.gen.Store(newGeneration(changed, 2, nil))
	got := request.DocumentSymbols(uriFromPath(idx.Types[0].File))
	if len(got) != 1 || got[0].Name != idx.Types[0].Name {
		t.Fatalf("request mixed generations: %#v", got)
	}
	if got := h.DocumentSymbols(uriFromPath(idx.Types[0].File)); len(got) != 1 || got[0].Name != "Replacement" {
		t.Fatalf("new request did not see new generation: %#v", got)
	}
}
