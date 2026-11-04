package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Case-insensitive record field identity is a local DML contract.
func TestFlowInsertCanonicalFieldsBeforeAutomation(t *testing.T) {
	for _, field := range []string{"name", "NAME"} {
		t.Run(field, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Before", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "contains", Value: "seed"}}, FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "changed"}}}}
			org.Objects["Account"] = object
			engine := NewEngine(&org)
			results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{field: storage.StringValue("seed")}}})
			if !results[0].Success {
				t.Fatalf("insert: %+v", results[0])
			}
			got := org.Objects["Account"].Records[results[0].ID].Fields["Name"].String
			if got != "changed" {
				t.Fatalf("Name=%q, want changed", got)
			}
		})
	}
}
