package enterprise

import (
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func TestSummarizeTrace(t *testing.T) {
	events := []trace.Event{
		trace.Instant("apex.soql", "apex.soql", 1, nil),
		trace.Instant("apex.dml.insert", "apex.dml", 2, nil),
		trace.Instant("apex.async.enqueue", "apex.async", 3, nil),
		trace.Instant("apex.callout.http", "apex.callout", 4, nil),
	}
	got := SummarizeTrace(events)
	if got.Events != 4 || got.SOQLStatements != 1 || got.DMLOperations != 1 || got.AsyncEvents != 1 || got.Callouts != 1 {
		t.Fatalf("summary = %#v", got)
	}
	if got.ByCategory["apex.soql"] != 1 {
		t.Fatalf("category counts = %#v", got.ByCategory)
	}
}
