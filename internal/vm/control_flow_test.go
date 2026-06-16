package vm

import "testing"

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
