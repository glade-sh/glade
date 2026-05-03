package profile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/trace"
)

func TestAnalyzeRanksTraceEvents(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Instant("apex.statement.expr", "apex.statement", 1, map[string]any{"sourceOffset": 10, "line": 2, "column": 3}),
		trace.Instant("apex.statement.expr", "apex.statement", 2, map[string]any{"sourceOffset": 20, "line": 3, "column": 5}),
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
	if got := report.Hot[0].SourceRanges; len(got) != 2 || got[0].Line != 2 || got[1].Column != 5 {
		t.Fatalf("ranges = %#v", got)
	}
}

func TestAnalyzeAttributesExpandedRuntimeEvents(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Instant("apex.soql", "apex.soql", 1, map[string]any{"rows": 3}),
		trace.Instant("apex.dml.insert", "apex.dml", 2, map[string]any{"rows": 2}),
		trace.Instant("apex.callout.http", "apex.callout", 3, nil),
		trace.Instant("apex.email.send", "apex.email", 4, nil),
		trace.Instant("apex.async.enqueue", "apex.async", 5, map[string]any{"kind": "Queueable"}),
		trace.Instant("apex.trigger.AccountBeforeInsert", "apex.trigger", 6, map[string]any{"rows": 2}),
		trace.Instant("apex.describe.sobject", "apex.describe", 7, map[string]any{"object": "Account"}),
		trace.Instant("apex.limits", "apex.limits", 8, map[string]any{
			"callouts":         1,
			"asyncJobs":        1,
			"queueableJobs":    1,
			"emailInvocations": 1,
			"cpuTimeMs":        9,
			"heapSize":         128,
		}),
	})

	report := Analyze(doc)
	if report.Categories["apex.trigger"] != 1 || report.Categories["apex.describe"] != 1 {
		t.Fatalf("categories = %#v", report.Categories)
	}
	if report.Limits.SOQLQueries != 1 || report.Limits.SOQLRows != 3 || report.Limits.DML != 1 || report.Limits.DMLRows != 2 {
		t.Fatalf("data limits = %#v", report.Limits)
	}
	if report.Limits.Callouts != 1 || report.Limits.AsyncJobs != 1 || report.Limits.QueueableJobs != 1 || report.Limits.EmailInvocations != 1 {
		t.Fatalf("platform limits = %#v", report.Limits)
	}
	if report.Limits.CPUTimeMS != 9 || report.Limits.HeapSize != 128 {
		t.Fatalf("resource limits = %#v", report.Limits)
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
