package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaNestedRelationshipUsesExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	prefs := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[prefs.APIName] = storage.ObjectState{Definition: prefs, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	parent := storage.Record{ID: "a01000000000001", Object: "Parent__c"}
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", KeyPrefix: "a01", Fields: map[string]storage.Field{"FlagFormula__c": {APIName: "FlagFormula__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Setup.Prefs__c.Flag__c"}}}, Records: map[storage.ID]storage.Record{parent.ID: parent}}
	childDef := storage.ObjectDefinition{APIName: "Child__c", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"}}}
	child := storage.Record{Object: "Child__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parent.ID)}}
	field := storage.Field{APIName: "Result__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "Parent__r.FlagFormula__c"}
	got, _, ok := EvaluateRecordFormulaValueInOrgWithOptions(field.Formula, field, &org, childDef, child, FormulaEvaluationOptions{UserID: actor})
	if !ok || got.Kind != storage.ValueBoolean || got.Boolean {
		t.Fatalf("nested formula=%#v supported=%v; want false", got, ok)
	}
}
