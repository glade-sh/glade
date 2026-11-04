package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFlowInsertDefaultBeforeCriteria(t *testing.T) {
	for _, tc := range []struct {
		name         string
		explicitNull bool
		want         string
	}{{"omitted", false, "seed"}} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Flag__c"] = storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "false"}
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Before", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: []storage.WorkflowCriteriaItem{{Field: "Flag__c", Operation: "isNull", Value: "true"}}, FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "changed"}}}}
			org.Objects["Account"] = object
			row := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("seed")}}
			if tc.explicitNull {
				row.ExplicitNulls = map[string]bool{"Flag__c": true}
			}
			engine := NewEngine(&org)
			got := engine.Insert([]storage.Record{row})[0]
			if !got.Success {
				t.Fatalf("insert: %+v", got)
			}
			name := org.Objects["Account"].Records[got.ID].Fields["Name"].String
			if name != tc.want {
				t.Fatalf("Name=%q, want %q", name, tc.want)
			}
		})
	}
}

// Salesforce captures the omitted false default before a record-triggered Flow
// evaluates its criterion; see testdata/flow_default_order/org.tsv.
func TestFlowInsertDefaultFalseBeforeCriteria(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value *bool
		want  string
	}{
		{"omitted", nil, "changed"},
		{"explicit-false", flowDefaultBoolPointer(false), "changed"},
		{"explicit-true", flowDefaultBoolPointer(true), "seed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			object.Definition.Fields["Flag__c"] = storage.Field{APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "false"}
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Before", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: []storage.WorkflowCriteriaItem{{Field: "Flag__c", Operation: "equals", Value: "false"}}, FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Name", LiteralValue: "changed"}}}}
			org.Objects["Account"] = object
			fields := map[string]storage.Value{"Name": storage.StringValue("seed")}
			if tc.value != nil {
				fields["Flag__c"] = storage.BooleanValue(*tc.value)
			}
			engine := NewEngine(&org)
			got := engine.Insert([]storage.Record{{Object: "Account", Fields: fields}})[0]
			if !got.Success {
				t.Fatalf("insert: %+v", got)
			}
			if name := org.Objects["Account"].Records[got.ID].Fields["Name"].String; name != tc.want {
				t.Fatalf("Name=%q, want %q", name, tc.want)
			}
		})
	}
}

func flowDefaultBoolPointer(value bool) *bool { return &value }
