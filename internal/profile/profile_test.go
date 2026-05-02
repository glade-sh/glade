package profile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/trace"
)

func TestAnalyzeRanksTraceEvents(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Instant("apex.statement.expr", "apex.statement", 1, map[string]any{"sourceOffset": 10}),
		trace.Instant("apex.statement.expr", "apex.statement", 2, map[string]any{"sourceOffset": 20}),
		trace.Instant("apex.statement.declare", "apex.statement", 0, map[string]any{"sourceOffset": 1}),
	})

	report := Analyze(doc)
	if report.Events != 3 || len(report.Hot) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if report.Hot[0].Name != "apex.statement.expr" || report.Hot[0].Count != 2 {
		t.Fatalf("hot[0] = %#v", report.Hot[0])
	}
	if got := report.Hot[0].SourceOffsets; len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("offsets = %#v", got)
	}
}

func TestWriteMarkdown(t *testing.T) {
	report := Report{Events: 1, Hot: []Entry{{Name: "apex.statement.expr", Category: "apex.statement", Count: 1, SourceOffsets: []int{5}}}}
	var out bytes.Buffer
	if err := WriteMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "| 1 | `apex.statement.expr` | `apex.statement` | 1 | [5] |") {
		t.Fatalf("markdown = %q", out.String())
	}
}

func TestReadTrace(t *testing.T) {
	doc, err := ReadTrace(strings.NewReader(`{"format":"chrome-trace-event","version":1,"traceEvents":[{"name":"x","ph":"i","ts":1,"pid":1,"tid":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Format != trace.FormatChromeTraceEvent || len(doc.TraceEvents) != 1 {
		t.Fatalf("doc = %#v", doc)
	}
}
