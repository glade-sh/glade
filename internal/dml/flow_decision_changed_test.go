package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce's Flow Operators reference defines Is Changed on a $Record field
// as a comparison with $Record__Prior. A create has no prior value and is false.
// https://help.salesforce.com/s/articleView?id=flow_ref_operators_condition.htm&type=5
func TestFlowDecisionChangedUsesPriorRecord(t *testing.T) {
	for _, trigger := range []string{"RecordBeforeSave", "RecordAfterSave"} {
		for _, tc := range []struct {
			name    string
			prior   *storage.Value
			current storage.Value
			operand string
			want    string
		}{
			{name: "created", current: storage.StringValue("Ready"), operand: "true", want: "Other"},
			{name: "created_false", current: storage.StringValue("Ready"), operand: "false", want: "Selected"},
			{name: "changed", prior: flowChangedTestValue(storage.StringValue("Draft")), current: storage.StringValue("Ready"), operand: "true", want: "Selected"},
			{name: "unchanged", prior: flowChangedTestValue(storage.StringValue("Ready")), current: storage.StringValue("Ready"), operand: "true", want: "Other"},
			{name: "changed_false", prior: flowChangedTestValue(storage.StringValue("Draft")), current: storage.StringValue("Ready"), operand: "false", want: "Other"},
			{name: "unchanged_false", prior: flowChangedTestValue(storage.StringValue("Ready")), current: storage.StringValue("Ready"), operand: "false", want: "Selected"},
			{name: "null_to_value", prior: flowChangedTestValue(storage.NullValue()), current: storage.StringValue("Ready"), operand: "true", want: "Selected"},
			{name: "value_to_null", prior: flowChangedTestValue(storage.StringValue("Ready")), current: storage.NullValue(), operand: "true", want: "Selected"},
			{name: "null_unchanged", prior: flowChangedTestValue(storage.NullValue()), current: storage.NullValue(), operand: "true", want: "Other"},
		} {
			t.Run(trigger+"/"+tc.name, func(t *testing.T) {
				org := testOrg()
				object := org.Objects["Account"]
				object.Definition.Fields["Phase__c"] = storage.Field{APIName: "Phase__c", Type: storage.FieldString}
				object.Definition.Fields["Outcome__c"] = storage.Field{APIName: "Outcome__c", Type: storage.FieldString}
				rule := storage.FlowRule{Name: "RouteOnChange", Active: true, TriggerType: trigger, RecordTriggerType: "CreateAndUpdate"}
				object.Definition.FlowRules = []storage.FlowRule{rule}
				id := storage.ID("001000000000001")
				current := storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout"), "Phase__c": tc.current}}
				var prior storage.Record
				if tc.prior != nil {
					prior = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout"), "Phase__c": *tc.prior}}
					object.Records[id] = prior
				}
				org.Objects["Account"] = object
				engine := NewEngine(&org)
				if tc.prior == nil {
					if trigger == "RecordBeforeSave" {
						current.ID = ""
					} else {
						// Model the same explicit insert event as normal after-save execution.
						engine.RecordCreations.Record(id)
						engine.markCreatedAutomation(id)
						restore := engine.beginCreatedAutomation(id)
						defer restore()
					}
				}
				if trigger == "RecordAfterSave" && tc.prior != nil {
					engine.PriorRecords = map[storage.ID]storage.Record{id: prior}
				}
				steps := []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
					{Name: "Changed", Criteria: []storage.WorkflowCriteriaItem{{Field: "Phase__c", Operation: "ischanged", Value: tc.operand}}, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Selected"}}}}},
					{Name: "Other", Default: true, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Other"}}}}},
				}}}
				_, err := engine.applyFlowSteps(rule.Name, "Account", &current, object.Definition, steps, nil, nil)
				if err != nil {
					t.Fatalf("decision: %v", err)
				}
				if got := current.Fields["Outcome__c"].String; got != tc.want {
					t.Fatalf("outcome = %q, want %q", got, tc.want)
				}
			})
		}
	}
}

func flowChangedTestValue(value storage.Value) *storage.Value { return &value }

func TestFlowDecisionChangedKeepsOtherCriteria(t *testing.T) {
	org := testOrg()
	object := org.Objects["Account"]
	engine := NewEngine(&org)
	frame := newFlowFrame()
	frame.saveDecisions = true
	current := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout")}}
	for _, tc := range []struct {
		name, operation, value string
		matches, supported     bool
	}{
		{"equality", "equals", "Sprout", true, true},
		{"nonmatching_equality", "equals", "Other", false, true},
		{"unknown_operator", "notAnOperator", "true", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matches, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "Name", Operation: tc.operation, Value: tc.value}}}, current, object.Definition, frame)
			if matches != tc.matches || supported != tc.supported {
				t.Fatalf("got (%v,%v), want (%v,%v)", matches, supported, tc.matches, tc.supported)
			}
		})
	}
}
