package enterprise

import (
	"encoding/json"
	"testing"
)

func TestReportJSONShape(t *testing.T) {
	report := Report{
		SchemaVersion: SchemaVersion,
		Command:       "glade report assess --project .",
		Project: ProjectSummary{
			Root:        ".",
			ApexClasses: 2,
			Triggers:    1,
			Tests:       1,
		},
		Findings: []Finding{{
			ID:             "ENT-RISK-001",
			Category:       CategoryArchitecture,
			Severity:       SeverityHigh,
			Confidence:     ConfidenceMedium,
			Title:          "Trigger handler has high fan-out",
			Summary:        "AccountTriggerHandler has 12 downstream dependencies.",
			Symbol:         "AccountTriggerHandler",
			Location:       Location{File: "force-app/main/default/classes/AccountTriggerHandler.cls", LineStart: 10, LineEnd: 80},
			Evidence:       []Evidence{{Type: EvidenceGraph, Message: "12 outbound references found."}},
			Recommendation: "Add characterization tests before splitting the handler.",
			NextActions:    []string{"glade inspect graph --project . --json"},
			Tags:           []string{"trigger-path", "fan-out"},
		}},
	}
	report.RefreshSummary()

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if got["schema_version"] != SchemaVersion {
		t.Fatalf("schema_version = %v", got["schema_version"])
	}
	summary := got["summary"].(map[string]any)
	if summary["high"].(float64) != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if got["status"] != string(StatusWarn) {
		t.Fatalf("status = %v", got["status"])
	}
}

func TestSeverityAndConfidenceValidate(t *testing.T) {
	for _, severity := range []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo} {
		if !severity.Valid() {
			t.Fatalf("severity %q should be valid", severity)
		}
	}
	for _, confidence := range []Confidence{ConfidenceHigh, ConfidenceMedium, ConfidenceLow, ConfidenceUnknown} {
		if !confidence.Valid() {
			t.Fatalf("confidence %q should be valid", confidence)
		}
	}
	if Severity("urgent").Valid() {
		t.Fatalf("unexpected severity accepted")
	}
	if Confidence("certain").Valid() {
		t.Fatalf("unexpected confidence accepted")
	}
}
