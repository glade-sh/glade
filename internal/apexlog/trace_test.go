package apexlog

import (
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/trace"
)

func TestTraceDocumentConvertsMeasuredEvents(t *testing.T) {
	log := mustReadLogTestdata(t, "core.log")
	doc := TraceDocument(log)

	if doc.Format != trace.FormatChromeTraceEvent {
		t.Fatalf("doc format = %q, want %q", doc.Format, trace.FormatChromeTraceEvent)
	}

	var soqlEvent, dmlEvent, limitsEvent trace.Event
	foundSoql := false
	foundDml := false
	foundLimits := 0
	for _, event := range doc.TraceEvents {
		switch event.Name {
		case "apex.soql":
			soqlEvent = event
			foundSoql = true
		case "apex.dml.insert":
			dmlEvent = event
			foundDml = true
		case "apex.limits":
			limitsEvent = event
			foundLimits++
		}
	}
	if !foundSoql || !foundDml || foundLimits == 0 {
		t.Fatalf("missing measured event; foundSoql=%v foundDml=%v limits=%d", foundSoql, foundDml, foundLimits)
	}
	if rows, ok := soqlEvent.Args["rows"].(int); !ok || rows != 1 {
		t.Fatalf("soql rows = %#v", soqlEvent.Args["rows"])
	}
	if dmlRows, ok := dmlEvent.Args["rows"].(int); !ok || dmlRows != 1 {
		t.Fatalf("dml rows = %#v", dmlEvent.Args["rows"])
	}
	objects, _ := dmlEvent.Args["objects"].([]string)
	if len(objects) != 1 || objects[0] != "Account" {
		t.Fatalf("dml objects = %#v", objects)
	}

	if cpu, ok := limitsEvent.Args["cpuTimeMs"].(int); !ok || cpu != 7 {
		t.Fatalf("cpuTimeMs = %#v", limitsEvent.Args["cpuTimeMs"])
	}
	if heap, ok := limitsEvent.Args["heapSize"].(int); !ok || heap != 8120 {
		t.Fatalf("heapSize = %#v", limitsEvent.Args["heapSize"])
	}
}

func TestTraceDocumentIntegrationWithProfile(t *testing.T) {
	log := mustReadLogTestdata(t, "core.log")
	doc := TraceDocument(log)
	report := profile.Analyze(doc)
	if report.Limits.SOQLQueries != 1 {
		t.Fatalf("SOQLQueries = %d, want 1", report.Limits.SOQLQueries)
	}
	if report.Limits.SOQLRows != 1 {
		t.Fatalf("SOQLRows = %d, want 1", report.Limits.SOQLRows)
	}
	if report.Limits.DML != 1 {
		t.Fatalf("DML = %d, want 1", report.Limits.DML)
	}
	if report.Limits.DMLRows != 1 {
		t.Fatalf("DMLRows = %d, want 1", report.Limits.DMLRows)
	}
	if report.Limits.CPUTimeMS != 7 {
		t.Fatalf("CPUTimeMS = %d, want 7", report.Limits.CPUTimeMS)
	}
	if report.Limits.HeapSize != 8120 {
		t.Fatalf("HeapSize = %d, want 8120", report.Limits.HeapSize)
	}
}

func mustReadLogTestdata(t *testing.T, name string) Log {
	t.Helper()
	data, err := os.ReadFile(strings.Join([]string{"testdata", name}, "/"))
	if err != nil {
		t.Fatal(err)
	}
	log, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func mustReadApexLog(t *testing.T, name string) Log {
	t.Helper()
	return mustReadLogTestdata(t, name)
}
