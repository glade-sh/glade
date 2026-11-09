package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaHierarchyOwnerPrecedence(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name          string
			user, profile bool
			want          bool
		}{
			{"organization_only", false, false, true},
			{"user_over_org", true, false, false},
			{"profile_over_org", false, true, false},
			{"user_over_profile", true, true, false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				org.OrgID = "00D000000000mH2"
				user := storage.ID(storage.DefaultUserID)
				profile := storage.ID("00e000000000002")
				org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{user: {ID: user, Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profile)}}}}
				def := storage.ObjectDefinition{APIName: "Prefs__c", Label: "Prefs", KeyPrefix: "a02", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}, "SetupOwnerId": {APIName: "SetupOwnerId", Type: storage.FieldReference}, "Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
				org.Objects[def.APIName] = storage.ObjectState{Definition: def}
				engine := NewEngine(&org)
				rows := []storage.Record{{Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Organization"), "SetupOwnerId": storage.IDValue(storage.ID(org.OrgID)), "Flag__c": storage.BooleanValue(true)}}}
				if tc.profile {
					rows = append(rows, storage.Record{Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("Profile"), "SetupOwnerId": storage.IDValue(profile), "Flag__c": storage.BooleanValue(tc.user)}})
				}
				if tc.user {
					rows = append(rows, storage.Record{Object: def.APIName, Fields: map[string]storage.Value{"Name": storage.StringValue("User"), "SetupOwnerId": storage.IDValue(user), "Flag__c": storage.BooleanValue(false)}})
				}
				for _, row := range rows {
					result := engine.Insert([]storage.Record{row})
					if len(result) != 1 || !result[0].Success {
						t.Fatalf("insert: %+v", result)
					}
				}
				field := storage.Field{APIName: "Result__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}
				got, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, storage.ObjectDefinition{APIName: "Account"}, storage.Record{Object: "Account"}, FormulaEvaluationOptions{UserID: user})
				if !ok || got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
					t.Fatalf("formula = %#v supported=%v; want %v", got, ok, tc.want)
				}
			})
		}
	}
}

// The formula options carry the execution user; a different user's row must
// not mask that user's own hierarchy entry.
func TestSetupFormulaHierarchyExplicitActor(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	field := storage.Field{APIName: "Result__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}
	got, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, storage.ObjectDefinition{APIName: "Account"}, storage.Record{}, FormulaEvaluationOptions{UserID: actor})
	if !ok || got.Kind != storage.ValueBoolean || got.Boolean {
		t.Fatalf("formula = %#v supported=%v; want false", got, ok)
	}
}
