package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native API 62/67 REST and anonymous rows:
// testdata/conformance/case2692_checkbox_boundary/org.tsv:2-5,8-11,14-17,20-23.
func TestFlowCheckboxNullDefaultAndFilterBoundaries(t *testing.T) {
	for _, def := range []string{"false", "true"} {
		for _, name := range []string{"Safe", "Reject"} {
			t.Run(def+"/"+name, func(t *testing.T) {
				org := testOrg()
				object := org.Objects["Account"]
				object.Definition.Fields["Flag__c"] = storage.Field{APIName: "Flag__c", Label: "Flag", Type: storage.FieldBoolean, DefaultValue: def}
				object.Definition.FlowRules = []storage.FlowRule{{Name: "Reject", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: "equals", Value: "Reject"}}, Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Reject", Messages: []storage.FlowCustomErrorMessage{{Message: "flow rejected"}}}}}}}
				org.Objects["Account"] = object
				engine := NewEngine(&org)
				result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}, ExplicitNulls: map[string]bool{"Flag__c": true}}})[0]
				if result.StatusCode != "INVALID_TYPE_ON_FIELD_IN_RECORD" {
					t.Fatalf("insert=%+v", result)
				}
			})
		}
	}
}
