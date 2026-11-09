package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSetupFormulaExplicitNameOrganizationID(t *testing.T) {
	for _, ownerType := range []storage.FieldType{storage.FieldString, storage.FieldReference} {
		t.Run(string(ownerType), func(t *testing.T) {
			org := storage.NewOrgState()
			org.OrgID = "00D000000000mH2"
			definition := storage.ObjectDefinition{APIName: "Prefs__c", Label: "Prefs", KeyPrefix: "a02", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Type: storage.FieldString},
				"SetupOwnerId": {APIName: "SetupOwnerId", Type: ownerType},
				"Tone__c":      {APIName: "Tone__c", Type: storage.FieldString},
			}}
			org.Objects[definition.APIName] = storage.ObjectState{Definition: definition}
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{{Object: definition.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Chosen"), "Tone__c": storage.StringValue("bright")}}})
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("insert: %+v", result)
			}
			field := storage.Field{APIName: "Result__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "$Setup.Prefs__c.Tone__c"}
			value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, storage.ObjectDefinition{APIName: "Account"}, storage.Record{Object: "Account"})
			if !ok || value.Kind != storage.ValueString || value.String != "bright" {
				t.Fatalf("formula = %#v, ok=%v; want bright", value, ok)
			}
		})
	}
}

// TestSetupFormulaStoredOwnerIDKind covers restored reference values as well as
// strings written by the insert path.
func TestSetupFormulaStoredOwnerIDKind(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000mH2"
	def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Tone__c": {APIName: "Tone__c", Type: storage.FieldString}}}
	field := storage.Field{APIName: "Result__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "$Setup.Prefs__c.Tone__c"}
	for _, tc := range []struct {
		name  string
		owner storage.Value
		want  string
	}{
		{"string18", storage.StringValue(storage.ID18(org.OrgID)), "bright"},
		{"id18", storage.IDValue(storage.ID(storage.ID18(org.OrgID))), "bright"},
		{"id15", storage.IDValue(storage.ID(org.OrgID)), "bright"},
		{"other", storage.IDValue("005000000000001"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{"a02000000000001": {Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Chosen"), "SetupOwnerId": tc.owner, "Tone__c": storage.StringValue("bright")}}}}
			value, _, ok := EvaluateRecordFormulaValueInOrg(field.Formula, field, &org, storage.ObjectDefinition{APIName: "Account"}, storage.Record{Object: "Account"})
			if !ok || (tc.want == "" && value.Kind != storage.ValueNull) || (tc.want != "" && (value.Kind != storage.ValueString || value.String != tc.want)) {
				t.Fatalf("formula = %#v, ok=%v; want %q", value, ok, tc.want)
			}
		})
	}
}
