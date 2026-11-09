package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Rule shapes without native response-rule evidence remain unsupported.
func TestB3AutoResponseUncapturedCriteriaRemainUnsupported(t *testing.T) {
	for _, entry := range []storage.AutoResponseRuleEntry{
		{Criteria: []storage.WorkflowCriteriaItem{{Field: "Lead.Company", Operation: "contains", Value: "Syn"}}},
		{Formula: `BEGINS(Company, "Syn")`},
		{BooleanFilter: "1 OR 2", Criteria: []storage.WorkflowCriteriaItem{{Field: "Lead.Company", Operation: "equals", Value: "Synthetic"}, {Field: "Lead.LastName", Operation: "equals", Value: "Other"}}},
	} {
		org := storage.NewOrgState()
		storage.EnsureStandardObject(&org, "Lead")
		lead := org.Objects["Lead"]
		entry.Template = "Synthetic"
		entry.Subject = "Synthetic"
		lead.Definition.AutoResponseRules = []storage.AutoResponseRule{{Name: "Synthetic", Active: true, Entries: []storage.AutoResponseRuleEntry{entry}}}
		id := storage.ID("00Q000000000001")
		lead.Records[id] = storage.Record{Object: "Lead", ID: id, Fields: map[string]storage.Value{"Company": storage.StringValue("Synthetic"), "LastName": storage.StringValue("Synthetic")}}
		org.Objects["Lead"] = lead
		engine := NewEngine(&org)
		if _, _, err := engine.AutoResponseSubject(storage.Record{Object: "Lead", ID: id}); err == nil {
			t.Errorf("uncaptured criteria unexpectedly accepted: %+v", entry)
		}
	}
}
