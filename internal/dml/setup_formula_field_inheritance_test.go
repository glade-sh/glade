package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaHierarchyFieldInheritance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct {
			name          string
			user, profile storage.Value
			want          string
		}{
			{"field_user", storage.StringValue("user"), storage.StringValue("profile"), "user"},
			{"field_profile", storage.NullValue(), storage.StringValue("profile"), "profile"},
			{"field_org", storage.NullValue(), storage.NullValue(), "org"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = api
				org.OrgID = "00D000000000mH2"
				user := storage.ID(storage.DefaultUserID)
				profile := storage.ID("00e000000000002")
				org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{user: {ID: user, Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profile)}}}}
				def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Tone__c": {APIName: "Tone__c", Type: storage.FieldString}}}
				org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{
					"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(org.OrgID)), "Tone__c": storage.StringValue("org")}},
					"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(profile), "Tone__c": tc.profile}},
					"a02000000000003": {ID: "a02000000000003", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(user), "Tone__c": tc.user}},
				}}
				field := storage.Field{APIName: "Result__c", Type: storage.FieldCalculated, DisplayType: "STRING", Formula: "$Setup.Prefs__c.Tone__c"}
				got, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, storage.ObjectDefinition{APIName: "Account"}, storage.Record{}, FormulaEvaluationOptions{UserID: user})
				if !ok || got.Kind != storage.ValueString || got.String != tc.want {
					t.Fatalf("formula=%#v supported=%v; want %q", got, ok, tc.want)
				}
			})
		}
	}
}
