package probe

import (
	"os"
	"path/filepath"
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
