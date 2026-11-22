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
	left := storage.WorkflowCriteriaItem{Field: "Parent.Name", Operation: "equals", SourceField: "Parent.Name"}
	right := storage.WorkflowCriteriaItem{Field: "Name", Operation: "equals", Value: "value"}
	for _, tc := range []struct {
		name   string
		lookup storage.FlowRecordLookup
		want   bool
	}{
		{"captured", storage.FlowRecordLookup{GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, true},
		{"one_filter", storage.FlowRecordLookup{GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left}}, false},
		{"or", storage.FlowRecordLookup{GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "or", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
		{"different_source", storage.FlowRecordLookup{GetFirstRecordOnly: true, StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{{Field: "Parent.Name", Operation: "equals", SourceField: "Name"}, right}}, false},
		{"collection", storage.FlowRecordLookup{StoreOutputAutomatically: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
		{"assign_null", storage.FlowRecordLookup{GetFirstRecordOnly: true, StoreOutputAutomatically: true, AssignNullValuesIfNoRecordsFound: true, CriteriaLogic: "and", Criteria: []storage.WorkflowCriteriaItem{left, right}}, false},
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
