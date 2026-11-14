package lsp

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestUnreflectedSymbolsAndHover(t *testing.T) {
	original := "public class First { void removed() {} }"
	second := "public class Second { void run() { First x; } }"
	seed, uris := overlayTestHandler(t, map[string]string{"First": original, "Second": second})
	h := NewPendingHandler()
	h.SetBase(Base{Index: seed.gen.Load().index, Project: project.Project{Root: t.TempDir()}})
	uri := uris["First"]
	overlayOpen(h, uri, original)
	if len(h.DocumentSymbols(uri)) == 0 || h.Hover(HoverParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: 27}}) == nil {
		t.Fatal("reflected document lost symbols or hover")
	}
	overlayChange(t, h, uri, "public class First { void added() { removed(); } }")
	if got := h.DocumentSymbols(uri); len(got) != 0 {
		t.Errorf("stale symbols: %#v", got)
	}
	if got := h.Hover(HoverParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: 27}}); got != nil {
		t.Errorf("stale hover: %#v", got)
	}
	h.analysisResult()
	if got := h.HoverForName("First", nil); got != nil {
		t.Errorf("stale named hover: %#v", got)
	}
	if got := h.Definition(DefinitionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: 37}}); len(got) != 0 {
		t.Errorf("stale definition: %#v", got)
	}
	for _, symbol := range h.WorkspaceSymbols("First") {
		if strings.EqualFold(symbol.Name, "First") {
			t.Errorf("stale workspace symbol: %#v", symbol)
		}
	}
	if got := h.SemanticTokensFull(SemanticTokensParams{TextDocument: TextDocumentIdentifier{URI: uri}}); len(got.Data) != 0 {
		t.Errorf("stale tokens: %#v", got)
	}
	if got := h.DocumentSymbols(uris["Second"]); len(got) != 1 || got[0].Name != "Second" {
		t.Errorf("unchanged neighbor: %#v", got)
	}
	overlayOpen(h, uris["Second"], second)
	if got := h.Definition(DefinitionParams{TextDocument: TextDocumentIdentifier{URI: uris["Second"]}, Position: Position{Character: 36}}); len(got) != 0 {
		t.Errorf("definition into stale buffer: %#v", got)
	}
	overlayChange(t, h, uri, original)
	if got := h.DocumentSymbols(uri); len(got) != 1 || len(got[0].Children) != 1 || got[0].Children[0].Name != "removed" {
		t.Errorf("reverted symbols: %#v", got)
	}
}
