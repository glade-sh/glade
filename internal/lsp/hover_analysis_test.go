package lsp

import (
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestHoverDoesNotWaitForAnalysis(t *testing.T) {
	idx := sampleIndex(t)
	idx.Types[0].Members[0].Type = "String"
	started, release := make(chan struct{}), make(chan struct{})
	h := NewPendingHandler()
	h.SetAnalyzer(func(typesys.Index) sema.Result {
		close(started)
		<-release
		return sema.Result{Types: map[string]sema.TypeReference{"string": {Name: "String", Kind: sema.TypeBuiltin}}}
	})
	h.SetIndex(idx)
	h.StartAnalysis()
	<-started
	done := make(chan *Hover, 1)
	go func() {
		done <- h.Hover(HoverParams{TextDocument: TextDocumentIdentifier{URI: uriFromPath(idx.Types[0].File)}, Position: Position{Line: 1, Character: 8}})
	}()
	select {
	case got := <-done:
		if got == nil {
			t.Error("missing fallback hover")
		}
	case <-time.After(time.Second):
		t.Error("hover waited for analysis")
	}
	close(release)
	h.analysisResult()
	got := h.Hover(HoverParams{TextDocument: TextDocumentIdentifier{URI: uriFromPath(idx.Types[0].File)}, Position: Position{Line: 1, Character: 8}})
	if got == nil || !strings.Contains(got.Contents.Value, "**String**") {
		t.Errorf("completed hover = %#v", got)
	}
}
