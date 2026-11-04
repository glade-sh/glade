package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native: ../apextest/testdata/conformance/flow_partial_rows/org.tsv.
func TestFlowCheckboxNullValidationBeforeFlow(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		explicit    bool
		code        string
	}{
		{"null-map", "Flag__c", true, "INVALID_TYPE_ON_FIELD_IN_RECORD"},
		{"null-value", "Other__c", false, "INVALID_TYPE_ON_FIELD_IN_RECORD"},
		{"omitted", "Flag__c", false, "FIELD_CUSTOM_VALIDATION_EXCEPTION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			for _, name := range []string{"Flag__c", "Other__c"} {
				object.Definition.Fields[name] = storage.Field{APIName: name, Label: "Probe Flag", Type: storage.FieldBoolean, DefaultValue: "false"}
			}
			object.Definition.FlowRules = []storage.FlowRule{{Name: "Reject", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create", Steps: []storage.FlowStep{{Kind: "customError", CustomError: storage.FlowCustomError{Name: "Reject", Messages: []storage.FlowCustomErrorMessage{{Message: "flow rejected"}}}}}}}
			org.Objects["Account"] = object
			row := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("seed")}}
			if tc.explicit {
				row.ExplicitNulls = map[string]bool{tc.field: true}
			} else if tc.name == "null-value" {
				row.Fields[tc.field] = storage.NullValue()
			}
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{row})[0]
			if result.Success || result.StatusCode != tc.code {
				t.Fatalf("got %+v, want %s", result, tc.code)
			}
			if tc.code == "INVALID_TYPE_ON_FIELD_IN_RECORD" && result.Error != "Probe Flag: value not of required type: " {
				t.Fatalf("message %q", result.Error)
			}
		})
	}
}
