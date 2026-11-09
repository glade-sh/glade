package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestSetupFormulaFlowCriteriaKeepFieldTypes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount string
		phrase string
		want   bool
	}{{"actor", "10", "NULL", true}, {"control", "1", "other", false}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			actor := storage.ID("005000000000002")
			def := storage.ObjectDefinition{APIName: "Prefs__c", Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"}, Fields: map[string]storage.Field{"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal}, "Phrase__c": {APIName: "Phrase__c", Type: storage.FieldString}}}
			org.Objects[def.APIName] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{"a02000000000001": {ID: "a02000000000001", Fields: map[string]storage.Value{"SetupOwnerId": storage.IDValue(actor), "Amount__c": storage.DecimalValue(tc.amount), "Phrase__c": storage.StringValue(tc.phrase)}}}}
			engine := NewEngine(&org)
			engine.UserID = actor
			for _, item := range []storage.WorkflowCriteriaItem{{Field: "$Setup.Prefs__c.Amount__c", Operation: "greaterThan", Value: "2"}, {Field: "$Setup.Prefs__c.Phrase__c", Operation: "equals", Value: "NULL"}} {
				got, ok := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{item}}, storage.Record{}, storage.ObjectDefinition{}, newFlowFrame())
				if !ok || got != tc.want {
					t.Errorf("%s got=%v supported=%v want=%v", item.Field, got, ok, tc.want)
				}
			}
		})
	}
}
