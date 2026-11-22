package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowReferenceIDLookupVisibilityScope(t *testing.T) {
	definition := storage.ObjectDefinition{Fields: map[string]storage.Field{
		"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		"Text__c":   {APIName: "Text__c", Type: storage.FieldString},
	}}
	criterion := storage.WorkflowCriteriaItem{Field: "Id", Operation: "equals", SourceField: "Parent__c"}
	for _, tc := range []struct {
		name   string
		lookup storage.FlowRecordLookup
		target string
		want   bool
	}{
		{"reference", storage.FlowRecordLookup{GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{criterion}}, "Account", true},
		{"other_target", storage.FlowRecordLookup{GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{criterion}}, "Contact", false},
		{"multiple", storage.FlowRecordLookup{GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{criterion, criterion}}, "Account", false},
		{"collection", storage.FlowRecordLookup{Criteria: []storage.WorkflowCriteriaItem{criterion}}, "Account", false},
		{"non_reference", storage.FlowRecordLookup{GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "equals", SourceField: "Text__c"}}}, "Account", false},
		{"other_operator", storage.FlowRecordLookup{GetFirstRecordOnly: true, Criteria: []storage.WorkflowCriteriaItem{{Field: "Id", Operation: "notequal", SourceField: "Parent__c"}}}, "Account", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := flowCapturedReferenceIDLookup(tc.lookup, definition, tc.target, ""); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlowRelationshipFilterFaultRuntimeScope(t *testing.T) {
	left := storage.WorkflowCriteriaItem{Field: "Name", Operation: "equals", SourceField: "Parent.Name"}
	right := storage.WorkflowCriteriaItem{Field: "Name", Operation: "equals", Value: "value"}
	for _, tc := range []struct {
		name   string
		lookup storage.FlowRecordLookup
		want   bool
	}{
		{"captured", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, true},
		{"one_filter", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left}}, false},
		{"or", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "or", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
		{"different_source", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", SourceField: "Name"}, right}}, false},
		{"collection", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
		{"assign_null", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", GetFirstRecordOnly: true, StoreOutputAutomatically: true, AssignNullValuesIfNoRecordsFound: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
		{"sort_order", storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent.Name", GetFirstRecordOnly: true, StoreOutputAutomatically: true, SortOrder: "Asc", CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := flowCapturedRelationshipFilterRuntime(tc.lookup); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlowRelationshipLookupFailureResultScope(t *testing.T) {
	rule := storage.FlowRule{Name: "Probe", TriggerType: "RecordAfterSave"}
	captured := flowCoreRecordTriggerError(rule, &flowRelationshipLookupFilterError{})
	result, ok := FlowRelationshipLookupFailureResult(captured)
	if !ok || result.Success || result.StatusCode != "CANNOT_EXECUTE_FLOW_TRIGGER" {
		t.Fatalf("captured result=%+v ok=%v", result, ok)
	}
	other := flowCoreRecordTriggerError(rule, &flowCoreUnknownSubflowOutputError{})
	if _, ok := FlowRelationshipLookupFailureResult(other); ok {
		t.Fatal("unrelated flow error matched")
	}
}

func TestFlowRelationshipFilterFieldTypesScope(t *testing.T) {
	for _, tc := range []struct {
		name        string
		parentType  storage.FieldType
		siblingType storage.FieldType
		targets     []string
		want        bool
	}{
		{"text", storage.FieldString, storage.FieldString, []string{"Parent__c"}, true},
		{"parent_boolean", storage.FieldBoolean, storage.FieldString, []string{"Parent__c"}, false},
		{"sibling_boolean", storage.FieldString, storage.FieldBoolean, []string{"Parent__c"}, false},
		{"polymorphic", storage.FieldString, storage.FieldString, []string{"Parent__c", "Other__c"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.OrgState{Objects: map[string]storage.ObjectState{"Parent__c": {Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: tc.parentType, DisplayType: "STRING"}}}}}}
			definition := storage.ObjectDefinition{APIName: "Child__c", Fields: map[string]storage.Field{
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, RelationshipName: "Parent__r", ReferenceTo: tc.targets},
				"Name":      {APIName: "Name", Type: tc.siblingType, DisplayType: "STRING"},
			}}
			lookup := storage.FlowRecordLookup{CapturedRelationshipFilterPath: "Parent__r.Name", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", SourceField: "Parent__r.Name"}, {Field: "Name", Value: "value"}}}
			if got := flowCapturedRelationshipFilterFields(&org, definition, lookup); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
