package probe

import "testing"

func TestSummarizeDebugLogs(t *testing.T) {
	logs := []ProbeDebugLog{
		{
			Phase:   "golden",
			ProbeID: "stdlib.string.valueof-null",
			Mode:    "single",
			Log: "11:00:00.0 (100)|METHOD_ENTRY|[1]|Foo.bar()\n" +
				"11:00:00.0 (200)|STATEMENT_EXECUTE|[2]\n" +
				"11:00:00.0 (300)|METHOD_ENTRY|[3]|Foo.baz()\n" +
				"11:00:00.0 (400)|USER_DEBUG|[4]|DEBUG|OAER_PROBE",
		},
	}
	summaries := SummarizeDebugLogs(logs)
	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	s := summaries[0]
	if s.TotalLines != 4 {
		t.Fatalf("totalLines = %d, want 4", s.TotalLines)
	}
	if got := s.Events["METHOD_ENTRY"]; got != 2 {
		t.Fatalf("METHOD_ENTRY count = %d, want 2", got)
	}
	if got := s.Events["STATEMENT_EXECUTE"]; got != 1 {
		t.Fatalf("STATEMENT_EXECUTE count = %d, want 1", got)
	}
	if got := s.Events["USER_DEBUG"]; got != 1 {
		t.Fatalf("USER_DEBUG count = %d, want 1", got)
	}
	if s.Signature == "" {
		t.Fatalf("signature should not be empty")
	}
}

func TestSummarizeDebugLogsStableSignature(t *testing.T) {
	entry := ProbeDebugLog{
		Phase: "golden",
		Mode:  "single",
		Log: "11:00:00.0 (100)|METHOD_ENTRY|[1]|Foo.bar()\n" +
			"11:00:00.0 (200)|STATEMENT_EXECUTE|[2]",
	}
	a := SummarizeDebugLogs([]ProbeDebugLog{entry})[0].Signature
	b := SummarizeDebugLogs([]ProbeDebugLog{entry})[0].Signature
	if a != b {
		t.Fatalf("signature mismatch: %q vs %q", a, b)
	}
}
