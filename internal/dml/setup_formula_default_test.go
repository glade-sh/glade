package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaTextDefaultUsesExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	prefs := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Tone__c": {APIName: "Tone__c", Type: storage.FieldString}}}
	org.Objects[prefs.APIName] = storage.ObjectState{Definition: prefs, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Tone__c": storage.StringValue("default")}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Tone__c": storage.StringValue("actor")}},
	}}
	def := storage.ObjectDefinition{APIName: "Example__c", KeyPrefix: "a01", Fields: map[string]storage.Field{"Tone__c": {APIName: "Tone__c", Type: storage.FieldString, DefaultValue: "$Setup.Prefs__c.Tone__c"}}}
	org.Objects[def.APIName] = storage.ObjectState{Definition: def}
	engine := NewEngine(&org)
	engine.UserID = actor
	result := engine.Insert([]storage.Record{{Object: def.APIName}})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("insert=%+v", result)
	}
	got := org.Objects[def.APIName].Records[result[0].ID].Fields["Tone__c"]
	if got.Kind != storage.ValueString || got.String != "actor" {
		t.Fatalf("default=%#v; want actor", got)
	}
}
