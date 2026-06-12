package enterprise

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	report := NewReport("glade report assess --project .", ProjectSummary{Root: ".", ApexClasses: 1})
	var buf bytes.Buffer
	if err := WriteJSON(&buf, report); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if !strings.Contains(buf.String(), "\n  ") {
		t.Fatalf("expected indented JSON, got %q", buf.String())
	}
	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q", decoded.SchemaVersion)
	}
}

func TestWriteMarkdownIncludesEvidence(t *testing.T) {
	report := NewReport("glade report cruft --project .", ProjectSummary{Root: ".", ApexClasses: 1})
	report.Findings = []Finding{{
		ID:             "ENT-CRUFT-001",
		Category:       CategoryCruft,
		Severity:       SeverityMedium,
		Confidence:     ConfidenceHigh,
		Title:          "Private method has no inbound references",
		Summary:        "LegacyDiscountService.oldPath has no inbound graph references.",
		Evidence:       []Evidence{{Type: EvidenceGraph, Message: "0 inbound references found."}},
		Recommendation: "Delete after affected tests pass.",
	}}
	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, report); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"# Glade Enterprise Report", "ENT-CRUFT-001", "0 inbound references found.", "Delete after affected tests pass."} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown missing %q:\n%s", want, out)
		}
	}
}

func TestWriteMarkdownIncludesTraceSummary(t *testing.T) {
	report := NewReport("glade report refactor-proof --project .", ProjectSummary{Root: "."})
	report.Trace = &TraceSummary{Events: 3, SOQLStatements: 1, DMLOperations: 1, AsyncEvents: 1, Callouts: 0}
	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, report); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if !strings.Contains(buf.String(), "Trace: events=3 soql=1 dml=1 async=1 callouts=0") {
		t.Fatalf("trace summary missing:\n%s", buf.String())
	}
}

func TestWriteHTMLEscapesFindingText(t *testing.T) {
	report := NewReport("glade report assess --project .", ProjectSummary{Root: "."})
	report.Findings = []Finding{{
		ID:             "ENT-RISK-HTML",
		Category:       CategoryArchitecture,
		Severity:       SeverityHigh,
		Confidence:     ConfidenceMedium,
		Title:          "<script>alert(1)</script>",
		Summary:        "Unsafe text must be escaped.",
		Evidence:       []Evidence{{Type: EvidenceHeuristic, Message: "Observed <tag> in title."}},
		Recommendation: "Escape report fields.",
	}}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, report); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Fatalf("HTML was not escaped:\n%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped title missing:\n%s", out)
	}
}

func TestWriteHTMLIncludesTraceSummary(t *testing.T) {
	report := NewReport("glade report refactor-proof --project .", ProjectSummary{Root: "."})
	report.Trace = &TraceSummary{Events: 4, SOQLStatements: 2}
	var buf bytes.Buffer
	if err := WriteHTML(&buf, report); err != nil {
		t.Fatalf("WriteHTML: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Trace Summary", "Events: 4", "SOQL: 2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("HTML missing %q:\n%s", want, out)
		}
	}
}
