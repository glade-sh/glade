package perfscan

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/trace"
)

func TestTraceScanAddsMeasuredFindings(t *testing.T) {
	doc := trace.NewDocument([]trace.Event{
		trace.Duration("apex.method.PerfRisk.uncachedAccounts", "apex.method", 0, 125000, map[string]any{"file": "PerfRisk.cls", "line": 3}),
		trace.Duration("apex.soql", "apex.soql", 126000, 35000, map[string]any{"query": "SELECT Id, Name FROM Account", "rows": 1000, "line": 5}),
	})
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	var report Report
	if err := scanTraceBytes(&report, data); err != nil {
		t.Fatal(err)
	}
	report.Finalize()

	assertFinding(t, report, "perf.measured.hot-span")
	assertFinding(t, report, "perf.measured.soql-rows")
	if len(report.Measurements) != 2 {
		t.Fatalf("measurements = %#v", report.Measurements)
	}
}
