package lsp

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStartAnalysisRunsOnceAndSeedsFirstRequest(t *testing.T) {
	var runs atomic.Int64
	started := make(chan struct{})
	handler := NewPendingHandler()
	handler.SetAnalyzer(func(typesys.Index) sema.Result {
		if runs.Add(1) == 1 {
			close(started)
		}
		return sema.Result{}
	})
	handler.SetIndex(sampleIndex(t))
	handler.StartAnalysis()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("analysis did not start before any request")
	}
	handler.analysisResult()
	handler.analysisResult()
	if got := runs.Load(); got != 1 {
		t.Fatalf("analyzer ran %d times, want 1", got)
	}
}
