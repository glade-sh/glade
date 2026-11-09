package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaStandardControllerUsesExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	id := storage.ID("001000000000001")
	prefs := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[prefs.APIName] = storage.ObjectState{Definition: prefs, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	account := storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{"SetupFlag__c": {APIName: "SetupFlag__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}}}
	org.Objects["Account"] = storage.ObjectState{Definition: account, Records: map[storage.ID]storage.Record{id: {ID: id, Object: "Account"}}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: actor, Object: "User"})
	record := Object("Account")
	record.Fields["Id"] = platformScalar("Id", string(id))
	got, err := machine.standardControllerFieldProjection(record, List(String("SetupFlag__c")))
	if err != nil {
		t.Fatal(err)
	}
	_, flag, ok := objectFieldValue(got, "SetupFlag__c")
	if !ok || flag.Kind != ValueBool || flag.Bool {
		t.Fatalf("projection=%#v want false", got)
	}
}
