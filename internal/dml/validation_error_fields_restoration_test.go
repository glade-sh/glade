package dml

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Restores declared rule-field identity. Native namespace spelling remains
// uncaptured; unrelated owners and undeclared fields keep their existing spelling.
func TestValidationErrorFieldsResolveDeclaredRuleNamespace(t *testing.T) {
	for _, tc := range []struct {
		name, namespace, display string
		want                     []string
	}{
		{"owner_local", "parcel", "Score__c", []string{"parcel__Score__c"}},
		{"owner_qualified", "parcel", "parcel__Score__c", []string{"parcel__Score__c"}},
		{"foreign_qualified", "parcel", "remote__Score__c", []string{"remote__Score__c"}},
		{"standard", "parcel", "Name", []string{"Name"}},
		{"record_level", "parcel", "", nil},
		{"unknown", "parcel", "Unknown__c", []string{"Unknown__c"}},
		{"no_owner", "", "Score__c", []string{"Score__c"}},
		{"unrelated_owner", "other", "Score__c", []string{"Score__c"}},
		{"local_declaration_only", "parcel", "Score__c", []string{"Score__c"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Namespace = "remote" // The invoking org does not own the rule.
			definition := storage.ObjectDefinition{
				APIName: "parcel__ScorePad__c",
				Fields: map[string]storage.Field{
					"Name":             {APIName: "Name", Type: storage.FieldString},
					"Score__c":         {APIName: "Score__c", Type: storage.FieldDecimal},
					"parcel__Score__c": {APIName: "parcel__Score__c", Type: storage.FieldDecimal},
					"remote__Score__c": {APIName: "remote__Score__c", Type: storage.FieldDecimal},
				},
				ValidationRules: []storage.ValidationRule{{
					Name: "RejectBlocked", Namespace: tc.namespace, Active: true,
					ErrorConditionFormula: `Name = "Blocked"`,
					ErrorMessage:          "rejected value",
					ErrorDisplayField:     tc.display,
				}},
			}
			if tc.name == "local_declaration_only" {
				delete(definition.Fields, "parcel__Score__c")
			}
			engine := NewEngine(&org)
			row := storage.Record{Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Blocked")}}
			for _, insert := range []bool{true, false} {
				err := engine.validateValidationRules(definition.APIName, definition, row, &row, insert)
				if err == nil {
					t.Fatal("expected validation failure")
				}
				result := resultFromError("", err)
				if result.Success || result.StatusCode != "FIELD_CUSTOM_VALIDATION_EXCEPTION" || result.Error != "rejected value" || !reflect.DeepEqual(result.Fields, tc.want) {
					t.Errorf("insert=%t: result=%#v; want fields=%v", insert, result, tc.want)
				}
			}
		})
	}
}
