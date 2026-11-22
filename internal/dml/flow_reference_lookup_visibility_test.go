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
