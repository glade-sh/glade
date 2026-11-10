package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowActionDateRefireScope(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Example__c", Fields: map[string]storage.Field{
		"Date__c": {APIName: "Date__c", Type: storage.FieldDate},
		"Time__c": {APIName: "Time__c", Type: storage.FieldDateTime},
		"Text__c": {APIName: "Text__c", Type: storage.FieldString},
	}}
	rule := storage.FlowRule{Steps: []storage.FlowStep{
		{Kind: "action", Action: storage.FlowAction{Name: "GetValue"}},
		{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Date__c", ActionOutput: "GetValue"}, {Field: "Time__c", ActionOutput: "GetValue"}}},
	}}
	if !flowActionDateRefireShape(rule, definition, "") {
		t.Fatal("captured action-output shape was excluded")
	}
	rule.Steps[1].FieldUpdates[1].Field = "Text__c"
	if flowActionDateRefireShape(rule, definition, "") {
		t.Fatal("unobserved field type must retain per-record refiring")
	}
	rule.Steps[1].FieldUpdates[1].Field = "Time__c"
	rule.Steps[1].FieldUpdates[1].ActionOutput = "Other"
	if flowActionDateRefireShape(rule, definition, "") {
		t.Fatal("a different action output must retain per-record refiring")
	}
}
