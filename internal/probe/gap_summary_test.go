package probe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSummarizeGaps(t *testing.T) {
	entries := []GapEntry{
		{
			ProbeID: "stub.date.today",
			GapType: GapTypeUnsupported,
			Diff:    "org returns 2024-01-02; local throws UnsupportedFeature",
		},
		{
			ProbeID: "stub.schema.describefieldresult.getname",
			GapType: GapTypeBehavioral,
			Diff:    "org throws System.CompileException; local returns {}",
		},
		{
			ProbeID: "dml.insert-trigger",
			GapType: GapTypeBehavioral,
			Diff:    "org returns true; local throws System.DmlException",
		},
	}
	summary := SummarizeGaps(entries)
	if summary.Total != 3 {
		t.Fatalf("total = %d, want 3", summary.Total)
	}
	if got := summary.ByGapType[string(GapTypeUnsupported)]; got != 1 {
		t.Fatalf("unsupported count = %d, want 1", got)
	}
	if got := summary.ByFamily["schema"]; got != 1 {
		t.Fatalf("schema family count = %d, want 1", got)
	}
	if got := summary.ByFamily["date"]; got != 1 {
		t.Fatalf("date family count = %d, want 1", got)
	}
	if got := summary.ByFamily["dml"]; got != 1 {
		t.Fatalf("dml family count = %d, want 1", got)
	}
	if len(summary.UnsupportedIDs) != 1 || summary.UnsupportedIDs[0] != "stub.date.today" {
		t.Fatalf("unsupported ids = %#v", summary.UnsupportedIDs)
	}
	if got := summary.ByDiffShape["return->throw:System.DmlException"]; got != 1 {
		t.Fatalf("dml diff shape count = %d, want 1", got)
	}
	if got := summary.StubSuperfamilyCounts["date"]; got != 1 {
		t.Fatalf("date stub superfamily count = %d, want 1", got)
	}
	if got := summary.StubSuperfamilyCounts["schema"]; got != 1 {
		t.Fatalf("schema stub superfamily count = %d, want 1", got)
	}
	if len(summary.DiffShapeTop) == 0 {
		t.Fatalf("diff shape top is empty")
	}
}

func TestSummarizeGapReport(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "gap-report.json")
	report := GapReport{
		Entries: []GapEntry{
			{ProbeID: "stub.date.dayofyear", GapType: GapTypeUnsupported, Diff: "org returns 1; local throws UnsupportedFeature"},
			{ProbeID: "stub.schema.describesobjectresult.getname", GapType: GapTypeBehavioral, Diff: "org throws System.CompileException; local returns {}"},
			{ProbeID: "dml.insert-trigger", GapType: GapTypeBehavioral, Diff: "org returns true; local throws System.DmlException"},
		},
	}
	if err := WriteReport(&report, reportPath); err != nil {
		t.Fatalf("write report: %v", err)
	}
	summary, err := SummarizeGapReport(reportPath)
	if err != nil {
		t.Fatalf("summarize report: %v", err)
	}
	if summary.Total != 3 {
		t.Fatalf("total = %d, want 3", summary.Total)
	}
	if got := len(summary.UnsupportedIDs); got != 1 {
		t.Fatalf("unsupported id count = %d, want 1", got)
	}
	_ = os.Remove(reportPath)
}

func TestSummarizeGapReportIncludesTraceClassificationCounts(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "gap-report-with-traces.json")
	raw := `{
  "entries": [
    {"probeId":"stub.connectapi-feed.get","gapType":"behavioral_gap","diff":"org throws System.UnsupportedOperationException; local throws System.CompileException"},
    {"probeId":"stub.connectapi-feed.post","gapType":"behavioral_gap","diff":"org throws System.UnsupportedOperationException; local returns <nil>"}
  ],
  "traceDiffs": [
    {"classification":"contract_equivalent"},
    {"classification":"contract_equivalent"},
    {"classification":"missing_trace"}
  ]
}`
	if err := os.WriteFile(reportPath, []byte(raw), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	summary, err := SummarizeGapReport(reportPath)
	if err != nil {
		t.Fatalf("summarize report: %v", err)
	}
	if got := summary.TraceClassificationCounts["contract_equivalent"]; got != 2 {
		t.Fatalf("contract_equivalent count = %d, want 2", got)
	}
	if got := summary.TraceClassificationCounts["missing_trace"]; got != 1 {
		t.Fatalf("missing_trace count = %d, want 1", got)
	}
	if got := summary.StubSuperfamilyCounts["connectapi"]; got != 2 {
		t.Fatalf("connectapi superfamily count = %d, want 2", got)
	}
	if len(summary.DiffShapeTop) < 2 {
		t.Fatalf("diff shape top = %#v, want at least 2 entries", summary.DiffShapeTop)
	}
	foundThrowToReturn := false
	for _, row := range summary.DiffShapeTop {
		if strings.HasPrefix(row.Name, "throw->return:System.UnsupportedOperationException") {
			foundThrowToReturn = true
			if row.Count != 1 {
				t.Fatalf("throw->return count = %d, want 1", row.Count)
			}
			break
		}
	}
	if !foundThrowToReturn {
		t.Fatalf("diff shape top missing throw->return shape: %#v", summary.DiffShapeTop)
	}
}
