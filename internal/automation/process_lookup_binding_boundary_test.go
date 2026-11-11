package automation

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestProcessBuilderLookupBindingBoundaries(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "process_builder_review_afterprojection", "project", "force-app", "main", "default", "flows", "Fern_Lookup_Sub.flow-meta.xml")
	flow, _, err := loadFlow(path)
	if err != nil {
		t.Fatal(err)
	}
	target := flow.Rules[0]
	action := storage.FlowAction{Inputs: []storage.WorkflowFieldUpdate{{Field: "recordId", SourceField: "Id"}, {Field: "rootId", SourceField: "Fern_Root_Link__r.Id"}}}
	defs := map[string]storage.ObjectDefinition{
		"Contact":      {Fields: map[string]storage.Field{"Fern_Root_Link__c": {APIName: "Fern_Root_Link__c", Type: storage.FieldReference, ReferenceTo: []string{"Fern_Root__c"}}, "Fern_Result__c": {APIName: "Fern_Result__c", Type: storage.FieldString, DisplayType: "RICHTEXTAREA"}}},
		"Fern_Root__c": {Fields: map[string]storage.Field{"Fern_Value__c": {APIName: "Fern_Value__c", Type: storage.FieldString, LongTextArea: true}}},
	}
	check := func(label string, want bool) {
		t.Helper()
		if got := processBuilderCapturedSubflow("Contact", action, target, defs); got != want {
			t.Errorf("%s: admitted=%v want %v", label, got, want)
		}
	}
	check("captured relationship ID", true)
	action.Inputs[1].SourceField = "Other__r.Id"
	check("different relationship", false)
	action.Inputs[1].SourceField = "Fern_Root_Link__r.Name"
	check("different target field", false)
	action.Inputs[1].SourceField = "Fern_Root_Link__r.Parent__r.Id"
	check("nested relationship", false)
}
