package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native rows: testdata/flow_name_order/org.tsv, Contact equals and IsNull routes.
func TestFlowInsertDerivedContactNameBeforeCriteria(t *testing.T) {
	for _, tc := range []struct{ first, operation, value, want string }{
		{"Ada", "equals", "Ada River", "other"}, {"Bea", "equals", "Ada River", "other"},
		{"Ada", "isNull", "true", "matched"}, {"Bea", "isNull", "false", "other"},
	} {
		t.Run(tc.first+"/"+tc.operation+"/"+tc.value, func(t *testing.T) {
			org := testOrg()
			org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
				"FirstName": {APIName: "FirstName", Type: storage.FieldString}, "LastName": {APIName: "LastName", Type: storage.FieldString, Required: true}, "Name": {APIName: "Name", Type: storage.FieldString}, "Description": {APIName: "Description", Type: storage.FieldString},
			}, FlowRules: []storage.FlowRule{{Name: "Before", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: tc.operation, Value: tc.value}}, FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Description", LiteralValue: "matched"}}}}}, Records: map[storage.ID]storage.Record{}}
			row := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"FirstName": storage.StringValue(tc.first), "LastName": storage.StringValue("River"), "Description": storage.StringValue("other")}}
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{row})[0]
			if !result.Success {
				t.Fatalf("insert: %+v", result)
			}
			record := org.Objects["Contact"].Records[result.ID]
			if got := record.Fields["Description"].String; got != tc.want {
				t.Fatalf("Description=%q, want %q", got, tc.want)
			}
			if got := record.Fields["Name"].String; got != tc.first+" River" {
				t.Fatalf("saved Name=%q", got)
			}
		})
	}
}
