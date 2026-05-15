package capability

import "testing"

func TestBuildStubBehaviorReportUsesStdlibEvidence(t *testing.T) {
	report := BuildStubBehaviorReport()
	if report.SchemaVersion != StubBehaviorSchemaVersion {
		t.Fatalf("schema version = %d", report.SchemaVersion)
	}
	if report.Totals.Entries == 0 || report.Totals.Members == 0 || report.Totals.Types == 0 {
		t.Fatalf("empty report totals: %+v", report.Totals)
	}
	if report.Totals.ByStatus[string(StubBehaviorImplemented)] == 0 || report.Totals.ByStatus[string(StubBehaviorPassiveDefault)] == 0 {
		t.Fatalf("missing expected status totals: %+v", report.Totals)
	}
	if report.Totals.ByStatus[string(StubBehaviorUnknown)] != 0 {
		t.Fatalf("unexpected unknown behavior entries: %+v", report.Totals)
	}

	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}
	stringTrim := findStubBehaviorEntry(entries, "String.trim(")
	if stringTrim == nil {
		t.Fatalf("missing String.trim entry")
	}
	if stringTrim.Status != StubBehaviorImplemented {
		t.Fatalf("String.trim status = %q", stringTrim.Status)
	}
	if len(stringTrim.Evidence) == 0 {
		t.Fatalf("String.trim missing evidence")
	}
	search := entries["Search"]
	if search.Status != StubBehaviorUnsupported {
		t.Fatalf("Search status = %q", search.Status)
	}
	pageCtor := findStubBehaviorEntry(entries, "PageReference.<init>(")
	if pageCtor == nil {
		t.Fatalf("missing PageReference constructor")
	}
	if pageCtor.Status != StubBehaviorImplemented {
		t.Fatalf("PageReference constructor status = %q", pageCtor.Status)
	}
}

func findStubBehaviorEntry(entries map[string]StubBehaviorEntry, prefix string) *StubBehaviorEntry {
	for id, entry := range entries {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			found := entry
			return &found
		}
	}
	return nil
}
