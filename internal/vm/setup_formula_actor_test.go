package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native runAs observations: setup_formula_org/org.tsv:18-19. The current
// user supplied to the VM is the local execution-context twin.
func TestSetupFormulaSOQLCurrentUser(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000mH2"
	user := storage.ID(storage.DefaultUserID)
	actor := storage.ID("005000000000002")
	setting := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[setting.APIName] = storage.ObjectState{Definition: setting, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Object: setting.APIName, Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(user), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Object: setting.APIName, Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	account := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"SetupFlag__c": {APIName: "SetupFlag__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}}}
	org.Objects["Account"] = storage.ObjectState{Definition: account, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account"}}}
	program, err := CompileAnonymous(`System.assertEquals(false,[SELECT SetupFlag__c FROM Account LIMIT 1].SetupFlag__c);`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: actor, Object: "User"})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	program, err = CompileAnonymous(`User alternate = new User(Id='005000000000002'); System.runAs(alternate) { System.assertEquals(false,[SELECT SetupFlag__c FROM Account LIMIT 1].SetupFlag__c); }`)
	if err != nil {
		t.Fatal(err)
	}
	machine.SetCurrentUser(storage.Record{ID: user, Object: "User"})
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSetupFormulaAPIsCurrentUser(t *testing.T) {
	org := storage.NewOrgState()
	org.OrgID = "00D000000000mH2"
	actor := storage.ID("005000000000002")
	setting := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[setting.APIName] = storage.ObjectState{Definition: setting, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Object: setting.APIName, Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Object: setting.APIName, Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	account := storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"SetupFlag__c": {APIName: "SetupFlag__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}}}
	org.Objects["Account"] = storage.ObjectState{Definition: account}
	program, err := CompileAnonymous(`
Account a=new Account();
List<FormulaRecalcResult> results=Formula.recalculateFormulas(new List<SObject>{a});
System.assert(results[0].isSuccess());
System.assertEquals(false,a.get('SetupFlag__c'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: actor, Object: "User"})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
