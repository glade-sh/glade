package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowBeforeSavePicklistChangeUsesImmediatePrior(t *testing.T) {
	// Owned before-save CreateAndUpdate observations use both Boolean operands
	// of IsChanged AND equality on this direct Picklist field. A prior insert
	// in the same transaction does not suppress a subsequent Picklist change.
	// String, after-save and Update-only cases below preserve existing local
	// behavior; those shapes have no matching native cells here.
	for _, tc := range []struct {
		name, prior, current, trigger, mode, operand, want string
		fieldType                                          storage.FieldType
		createdInTransaction                               bool
	}{
		{name: "committed_change_to_target", prior: "Cold", current: "Hot", want: "Selected"},
		{name: "committed_unchanged_target", prior: "Hot", current: "Hot", want: "Keep"},
		{name: "committed_change_to_other", prior: "Cold", current: "Warm", want: "Keep"},
		{name: "create_target", current: "Hot", want: "Keep"},
		{name: "same_transaction_change_to_target", prior: "Cold", current: "Hot", createdInTransaction: true, want: "Selected"},
		{name: "string_same_transaction_control", prior: "Cold", current: "Hot", fieldType: storage.FieldString, createdInTransaction: true, want: "Keep"},
		{name: "after_save_same_transaction_control", prior: "Cold", current: "Hot", trigger: "RecordAfterSave", createdInTransaction: true, want: "Keep"},
		{name: "same_transaction_false_operand", prior: "Cold", current: "Hot", operand: "false", createdInTransaction: true, want: "Keep"},
		{name: "update_only_same_transaction_control", prior: "Cold", current: "Hot", mode: "Update", createdInTransaction: true, want: "Keep"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			object := org.Objects["Account"]
			fieldType := tc.fieldType
			if fieldType == "" {
				fieldType = storage.FieldPicklist
			}
			object.Definition.Fields["State__c"] = storage.Field{APIName: "State__c", Type: fieldType}
			object.Definition.Fields["Outcome__c"] = storage.Field{APIName: "Outcome__c", Type: storage.FieldString}
			trigger, mode, operand := tc.trigger, tc.mode, tc.operand
			if trigger == "" {
				trigger = "RecordBeforeSave"
			}
			if mode == "" {
				mode = "CreateAndUpdate"
			}
			if operand == "" {
				operand = "true"
			}
			rule := storage.FlowRule{Name: "RoutePicklistChange", Active: true, TriggerType: trigger, RecordTriggerType: mode}
			object.Definition.FlowRules = []storage.FlowRule{rule}
			current := storage.Record{Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Meadow"), "State__c": storage.StringValue(tc.current), "Outcome__c": storage.StringValue("Keep"),
			}}
			var prior storage.Record
			if tc.prior != "" {
				current.ID = "001000000000001"
				prior = storage.Record{Object: "Account", ID: current.ID, Fields: map[string]storage.Value{
					"Name": storage.StringValue("Meadow"), "State__c": storage.StringValue(tc.prior), "Outcome__c": storage.StringValue("Keep"),
				}}
				object.Records[current.ID] = prior
			}
			org.Objects["Account"] = object
			engine := NewEngine(&org)
			if tc.createdInTransaction {
				engine.RecordCreations.Record(current.ID)
			}
			if trigger == "RecordAfterSave" && prior.ID != "" {
				engine.PriorRecords = map[storage.ID]storage.Record{prior.ID: prior}
			}
			steps := []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
				{Name: "ChangedToTarget", Criteria: []storage.WorkflowCriteriaItem{
					{Field: "State__c", Operation: "isChanged", Value: operand},
					{Field: "State__c", Operation: "equals", Value: "Hot"},
				}, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Selected"}}}}},
			}}}
			if _, err := engine.applyFlowSteps(rule.Name, "Account", &current, object.Definition, steps, nil, nil); err != nil {
				t.Fatal(err)
			}
			if got := current.Fields["Outcome__c"].String; got != tc.want {
				t.Fatalf("decision outcome=%q, want %q", got, tc.want)
			}
			if current.Fields["Name"].String != "Meadow" || current.Fields["State__c"].String != tc.current {
				t.Fatalf("decision changed an unassigned field: %#v", current.Fields)
			}
		})
	}
}
