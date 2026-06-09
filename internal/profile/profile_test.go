package profile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/trace"
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

func TestAnalyzeAggregatesDurationEvents(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Duration("apex.method.Slow.run", "apex.method", 100, 8000, map[string]any{"file": "Slow.cls", "line": 4}),
		trace.Duration("apex.method.Slow.run", "apex.method", 9000, 2000, map[string]any{"file": "Slow.cls", "line": 4}),
		trace.Instant("apex.soql", "apex.soql", 11000, map[string]any{"query": "SELECT Id FROM Account", "rows": 200}),
		trace.Duration("apex.soql", "apex.soql", 12000, 1500, map[string]any{"query": "SELECT Id FROM Account", "rows": 200}),
	})

	report := Analyze(doc)
	if len(report.Spans) == 0 {
		t.Fatalf("expected spans: %#v", report)
	}
	if report.Spans[0].Name != "apex.method.Slow.run" || report.Spans[0].DurationMS != 10 {
		t.Fatalf("top span = %#v", report.Spans[0])
	}
	if report.Spans[0].Count != 2 || report.Spans[0].SourceRanges[0].Line != 4 {
		t.Fatalf("span attribution = %#v", report.Spans[0])
	}
	if len(report.SOQL) != 1 || report.SOQL[0].Rows != 200 || report.SOQL[0].DurationCount != 1 || report.SOQL[0].Name != "SELECT Id FROM Account" {
		t.Fatalf("soql rows = %#v", report.SOQL)
	}
	if report.Limits.SOQLQueries != 1 || report.Limits.SOQLRows != 200 {
		t.Fatalf("soql limits = %#v", report.Limits)
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
	if len(report.SOQL) != 1 || len(report.DML) != 1 || len(report.Triggers) != 1 || len(report.Describe) != 1 || len(report.Callouts) != 1 || len(report.Async) != 1 || len(report.Platform) != 2 {
		t.Fatalf("report sections = %#v", report)
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

func TestAnalyzeAggregatesPostParitySurfaces(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Instant("apex.flow.rule", "apex.flow", 1, map[string]any{"flow": "TraceStatus", "matched": true}),
		trace.Instant("apex.flow.field_update", "apex.flow", 2, map[string]any{"field": "Status__c"}),
		trace.Instant("apex.visualforce.action.invoke", "apex.visualforce", 3, map[string]any{"className": "TraceController"}),
		trace.Instant("apex.visualforce.standard_controller.action.invoke", "apex.visualforce.standard_controller", 4, map[string]any{"object": "Account"}),
		trace.Instant("apex.metadata.deploy.enqueue", "apex.metadata", 5, map[string]any{"components": 1}),
		trace.Instant("apex.metadata.deploy.status", "apex.metadata", 6, map[string]any{"status": "SUCCEEDED"}),
	})

	report := Analyze(doc)
	if report.Categories["apex.flow"] != 2 || report.Categories["apex.visualforce"] != 1 || report.Categories["apex.metadata"] != 2 {
		t.Fatalf("categories = %#v", report.Categories)
	}
	if len(report.Automation) != 2 {
		t.Fatalf("automation = %#v", report.Automation)
	}
	if len(report.Visualforce) != 2 {
		t.Fatalf("visualforce = %#v", report.Visualforce)
	}
	if len(report.Metadata) != 2 {
		t.Fatalf("metadata = %#v", report.Metadata)
	}
}

func TestWriteMarkdown(t *testing.T) {
	report := Analyze(trace.NewDocument([]trace.Event{
		trace.Instant("apex.statement.expr", "apex.statement", 1, map[string]any{"sourceOffset": 5}),
		trace.Duration("apex.statement.expr", "apex.statement", 2, 1000, map[string]any{"sourceOffset": 5}),
		trace.Instant("apex.method.Work.run", "apex.method", 2, nil),
		trace.Instant("apex.soql", "apex.soql", 3, map[string]any{"rows": 4}),
		trace.Instant("apex.metadata.deploy.status", "apex.metadata", 5, map[string]any{"status": "SUCCEEDED"}),
		trace.Instant("apex.limits", "apex.limits", 4, map[string]any{"cpuTimeMs": 7, "heapSize": 64}),
	}))
	var out bytes.Buffer
	if err := WriteMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	markdown := out.String()
	if !strings.Contains(markdown, "## Runtime summary") || !strings.Contains(markdown, "CPU: 7 ms") || !strings.Contains(markdown, "Heap: 64 bytes") {
		t.Fatalf("markdown summary = %q", markdown)
	}
	for _, section := range []string{"## Categories", "## Measured spans", "## Hot events", "## Statements", "## Methods", "## SOQL", "## Platform", "## Metadata"} {
		if !strings.Contains(markdown, section) {
			t.Fatalf("markdown missing %s: %q", section, markdown)
		}
	}
	if !strings.Contains(markdown, "| 1 | `apex.statement.expr` | `apex.statement` | 2 | 0 | 1 | [5] |") {
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
