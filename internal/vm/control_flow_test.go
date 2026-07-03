package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func TestAppendTraceLazySkipsBuilderWhenTraceDisabled(t *testing.T) {
	called := false
	result := &Result{}
	appendTraceLazy(result, "apex.method.Example.run", "apex.method", func() map[string]any {
		called = true
		return map[string]any{"method": "Example.run"}
	})
	if called {
		t.Fatal("trace argument builder ran while tracing was disabled")
	}
	if len(result.Trace) != 0 {
		t.Fatalf("trace length = %d, want 0", len(result.Trace))
	}
}

func TestResultForLookupDoesNotCollectTrace(t *testing.T) {
	result := resultForLookup()
	if result.traceEnabled {
		t.Fatal("lookup result should not collect trace events")
	}
}

func TestAppendResultTraceCapsAndCoalesces(t *testing.T) {
	result := &Result{traceEnabled: true}
	for i := 0; i < maxTraceEvents+3; i++ {
		appendResultTrace(result, trace.Instant("apex.statement.expr", "apex.statement", int64(i), nil))
	}
	if len(result.Trace) != maxTraceEvents+1 {
		t.Fatalf("trace length = %d, want capped trace plus truncation event", len(result.Trace))
	}
	last := result.Trace[len(result.Trace)-1]
	if last.Name != "apex.trace.truncated" {
		t.Fatalf("last trace event = %#v, want truncation marker", last)
	}
	if got := last.Args["dropped"]; got != 3 {
		t.Fatalf("dropped trace count = %#v, want 3", got)
	}
}
