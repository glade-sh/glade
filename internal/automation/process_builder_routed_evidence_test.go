package automation

import "testing"

func TestProcessBuilderRoutedUpdateRequiresCapturedAssignment(t *testing.T) {
	update := flowRecordUpdateXML{FilterLogic: "1 AND (2 OR 3 OR 4 OR 5)", Filters: []flowFilterXML{
		{Field: "Id", Operator: "EqualTo", Value: flowValueXML{ElementReference: "$Record.Id"}},
		{Field: "Stage__c", Operator: "EqualTo", Value: flowValueXML{StringValue: "a"}},
		{Field: "Stage__c", Operator: "EqualTo", Value: flowValueXML{StringValue: "b"}},
		{Field: "Stage__c", Operator: "EqualTo", Value: flowValueXML{StringValue: "c"}},
		{Field: "Stage__c", Operator: "EqualTo", Value: flowValueXML{StringValue: "d"}},
	}}
	update.Fields = []flowFieldAssignmentXML{{Field: "Note__c", Value: flowValueXML{StringValue: "MATCH"}}}
	if !capturedProcessUpdateAlternatives(update) {
		t.Fatal("captured different-field update rejected")
	}
	update.Fields[0].Field = "Stage__c"
	if !capturedProcessUpdateAlternatives(update) {
		t.Fatal("captured update of filtered field rejected")
	}
}
