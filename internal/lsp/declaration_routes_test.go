package lsp

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func symbolChildren(t *testing.T, h *Handler, uri DocumentURI) []string {
	t.Helper()
	var names []string
	for _, symbol := range h.DocumentSymbols(uri) {
		for _, child := range symbol.Children {
			names = append(names, child.Name)
		}
	}
	return names
}

func completionHas(h *Handler, uri DocumentURI, character int, label string) bool {
	list := h.Completion(CompletionParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: character}})
	for _, item := range list.Items {
		if item.Label == label {
			return true
		}
	}
	return false
}

// Declaration routes answer from the text the current generation indexed.
// Only semantic diagnostics require the generation to reflect the buffer: a
// buffer that is mid-edit must not blank the outline, hover or completion.
func TestDeclarationRoutesFollowIndexedText(t *testing.T) {
	original := "public class First { void removed() {} }"
	h, uris := overlayTestHandler(t, map[string]string{"First": original})
	uri := uris["First"]
	overlayOpen(h, uri, original)
	overlayChange(t, h, uri, "public class First { void added() {} }")
	if got := symbolChildren(t, h, uri); len(got) != 1 || got[0] != "added" {
		t.Fatalf("edited symbols = %v, want [added]", got)
	}
	// A parse error keeps the last clean text.
	overlayChange(t, h, uri, "public class First { void added() { add } }")
	if got := symbolChildren(t, h, uri); len(got) != 1 || got[0] != "added" {
		t.Errorf("symbols during a parse error = %v, want [added]", got)
	}
	if !completionHas(h, uri, 39, "added") {
		t.Error("completion during a parse error lost the edited member")
	}
	if len(h.WorkspaceSymbols("First")) != 1 {
		t.Error("workspace symbol lost during a parse error")
	}
	if h.Hover(HoverParams{TextDocument: TextDocumentIdentifier{URI: uri}, Position: Position{Character: 27}}) == nil {
		t.Error("hover lost during a parse error")
	}
}

// A document the index does not know never reflects it. It must not hide the
// rest of the workspace.
func TestUnindexedDocumentsKeepWorkspaceDeclarations(t *testing.T) {
	original := "public class First { void removed() {} }"
	for _, unindexed := range []DocumentURI{uriFromPath(filepath.Join(t.TempDir(), "New.cls")), "untitled:Untitled-1"} {
		h, uris := overlayTestHandler(t, map[string]string{"First": original})
		overlayOpen(h, unindexed, "public class New {}")
		if len(h.WorkspaceSymbols("First")) != 1 || len(h.DocumentSymbols(uris["First"])) != 1 {
			t.Errorf("%s: workspace declarations withheld", unindexed)
		}
		if h.HoverForName("First", nil) == nil {
			t.Errorf("%s: hover withheld", unindexed)
		}
		if got := h.Definition(DefinitionParams{TextDocument: TextDocumentIdentifier{URI: uris["First"]}, Position: Position{Character: 15}}); len(got) != 1 {
			t.Errorf("%s: definition = %v", unindexed, got)
		}
	}
}

// A handler whose updater declines and that has no rebuild hook keeps its
// generation. Semantic diagnostics stop (the reflects rule); declaration
// routes keep answering from the indexed text, as they do without an overlay.
func TestDeclinedUpdateKeepsIndexedDeclarations(t *testing.T) {
	original := "public class First { void removed() {} }"
	seed, uris := overlayTestHandler(t, map[string]string{"First": original})
	h := NewPendingHandler()
	t.Cleanup(func() { h.parser.Close() })
	h.SetBase(Base{Index: seed.gen.Load().index, Project: project.Project{Root: t.TempDir()}})
	uri := uris["First"]
	overlayOpen(h, uri, original)
	before := h.gen.Load()
	if got := overlayChange(t, h, uri, "public class First { void added() {} }"); got != nil {
		t.Fatalf("published from an unreflected generation: %#v", got)
	}
	if h.gen.Load() != before {
		t.Fatal("declined update replaced the generation")
	}
	if got := symbolChildren(t, h, uri); len(got) != 1 || got[0] != "removed" {
		t.Errorf("symbols = %v, want the indexed [removed]", got)
	}
	if !completionHas(h, uri, 0, "removed") || h.HoverForName("First", nil) == nil {
		t.Error("indexed completion or hover withheld")
	}
}
