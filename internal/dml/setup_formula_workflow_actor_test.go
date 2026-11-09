package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaWorkflowUpdateUsesExecutionUser(t *testing.T) {
	org := storage.NewOrgState()
	actor := storage.ID("005000000000002")
	prefs := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean}}}
	org.Objects[prefs.APIName] = storage.ObjectState{Definition: prefs, Records: map[storage.ID]storage.Record{
		"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(storage.ID(storage.DefaultUserID)), "Flag__c": storage.BooleanValue(true)}},
		"a02000000000002": {ID: "a02000000000002", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Flag__c": storage.BooleanValue(false)}},
	}}
	id := storage.ID("a00000000000001")
	def := storage.ObjectDefinition{APIName: "Example__c", Fields: map[string]storage.Field{"Marked__c": {APIName: "Marked__c", Type: storage.FieldBoolean}}, WorkflowRules: []storage.WorkflowRule{{Name: "Rule", Active: true, Formula: "TRUE", FieldUpdates: []storage.WorkflowFieldUpdate{{Name: "Set", Field: "Marked__c", Formula: "$Setup.Prefs__c.Flag__c"}}}}}
	org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{id: {ID: id, Object: def.APIName, Fields: map[string]storage.Value{"Marked__c": storage.BooleanValue(true)}}}}
	engine := NewEngine(&org)
	engine.UserID = actor
	framed, ok := engine.flowContextFormulaValue("$Setup.Prefs__c.Flag__c", newFlowFrame())
	if !ok || framed.Kind != storage.ValueBoolean || framed.Boolean {
		t.Fatalf("framed formula=%#v supported=%v; want false", framed, ok)
	}
	branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "$Setup.Prefs__c.Flag__c", Operation: "equals", Value: "false"}}}
	if matched, ok := engine.evaluateFlowFrameBranch(branch, storage.Record{}, def, newFlowFrame()); !ok || !matched {
		t.Fatalf("flow criterion: matched=%v supported=%v", matched, ok)
	}
	if _, err := engine.applyWorkflowFieldUpdates(def.APIName, id); err != nil {
		t.Fatal(err)
	}
	got := org.Objects[def.APIName].Records[id].Fields["Marked__c"]
	if got.Kind != storage.ValueBoolean || got.Boolean {
		t.Fatalf("workflow value=%#v; want false", got)
	}
}
