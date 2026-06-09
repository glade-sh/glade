package perfscan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportModelSortsFindingsByScore(t *testing.T) {
	report := Report{SchemaVersion: SchemaVersion, Project: "/tmp/project"}
	report.AddFinding(Finding{
		ID:         "perf.apex.describe.repeated",
		Category:   CategoryDescribe,
		Severity:   SeverityMedium,
		Confidence: ConfidenceStatic,
		Score:      35,
		Message:    "Repeated describe calls can burn CPU and heap.",
		Location:   Location{File: "B.cls", Line: 2},
	})
	report.AddFinding(Finding{
		ID:         "perf.soql.loop",
		Category:   CategorySOQL,
		Severity:   SeverityHigh,
		Confidence: ConfidenceStatic,
		Score:      90,
		Message:    "SOQL inside a loop can exceed query limits.",
		Location:   Location{File: "A.cls", Line: 10},
	})

	report.Finalize()

	if report.Summary.Findings != 2 || report.Summary.High != 1 || report.Summary.Medium != 1 {
		t.Fatalf("summary = %#v", report.Summary)
	}
	if report.Findings[0].ID != "perf.soql.loop" {
		t.Fatalf("first finding = %#v", report.Findings[0])
	}
	if report.Summary.Categories[string(CategorySOQL)] != 1 {
		t.Fatalf("categories = %#v", report.Summary.Categories)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schemaVersion":1`) || !strings.Contains(string(data), `"confidence":"static"`) {
		t.Fatalf("json missing stable fields: %s", string(data))
	}
}

func TestMarkdownReportIncludesEvidenceAndFix(t *testing.T) {
	report := Report{SchemaVersion: SchemaVersion, Project: "/tmp/project"}
	report.AddFinding(Finding{
		ID:         "perf.soql.loop",
		Category:   CategorySOQL,
		Severity:   SeverityHigh,
		Confidence: ConfidenceStatic,
		Score:      90,
		EntryPoint: EntryPoint{Kind: EntryTrigger, Name: "AccountTrigger"},
		Message:    "SOQL inside a loop can exceed query limits.",
		Location:   Location{File: "force-app/main/default/classes/Selector.cls", Line: 12},
		Evidence: []Evidence{{
			Kind:    "apex",
			Message: "query executes inside loop depth 1",
		}},
		Fix: "Move the query outside the loop and use a keyed map.",
	})
	report.Finalize()

	var out strings.Builder
	if err := WriteMarkdown(&out, report); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"# Performance Scan",
		"Findings: 1",
		"`perf.soql.loop`",
		"AccountTrigger",
		"Selector.cls:12",
		"Move the query outside the loop",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown missing %q:\n%s", want, text)
		}
	}
}
