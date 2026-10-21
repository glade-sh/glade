package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce Flow Operators in Decision, Wait, and Collection Filter Elements:
// Is Changed is false on creation because there is no prior value to compare.
// https://help.salesforce.com/s/articleView?id=flow_ref_operators_condition.htm&language=en_US&type=5
func TestBeforeSaveCreateDecisionIsChanged(t *testing.T) {
	for _, tc := range []struct{ name, want, phase, marker string }{
		{"changedFalseOnCreate", "true", "Closed", "default"},
		{"unchangedTrueOnCreate", "false", "Closed", "matched"},
		{"andConditionStillApplies", "false", "Open", "default"},
		{"nullStillNew", "true", "", "default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			state := org.Objects["Account"]
			state.Definition.Fields["Phase__c"] = storage.Field{APIName: "Phase__c", Type: storage.FieldPicklist}
			state.Definition.Fields["Marker__c"] = storage.Field{APIName: "Marker__c", Type: storage.FieldString}
			mark := func(value string) []storage.FlowStep {
				return []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Marker__c", LiteralValue: value}}}}
			}
			state.Definition.FlowRules = []storage.FlowRule{{
				Name: "WatchPhase", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate",
				Steps: []storage.FlowStep{{Kind: "decision", Branches: []storage.FlowBranch{
					{Name: "ChangedAndClosed", Criteria: []storage.WorkflowCriteriaItem{
						{Field: "Phase__c", Operation: "isChanged", Value: tc.want},
						{Field: "Phase__c", Operation: "equals", Value: "Closed"},
					}, Steps: mark("matched")},
					{Name: "Default", Default: true, Steps: mark("default")},
				}}},
			}}
			org.Objects["Account"] = state
			rows := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Owned creation")}}}
			if tc.phase != "" {
				rows[0].Fields["Phase__c"] = storage.StringValue(tc.phase)
			}
			engine := NewEngine(&org)
			if err := engine.ApplyBeforeSaveFlows(rows); err != nil {
				t.Fatal(err)
			}
			if got := rows[0].Fields["Marker__c"].String; got != tc.marker {
				t.Fatalf("marker=%q want %q", got, tc.marker)
			}
		})
	}
}

func TestCreateDecisionIsChangedRetainsUnsupportedBoundaries(t *testing.T) {
	org := testOrg()
	definition := org.Objects["Account"].Definition
	definition.Fields["Phase__c"] = storage.Field{APIName: "Phase__c", Type: storage.FieldPicklist}
	engine := NewEngine(&org)
	frame := newFlowFrame()
	frame.saveDecisions = true
	for _, tc := range []struct {
		name, field, value string
		id                 storage.ID
	}{
		{name: "invalidField", field: "Missing__c", value: "true"},
		{name: "relationship", field: "Parent.Name", value: "true"},
		{name: "nonBoolean", field: "Phase__c", value: "unknown"},
		{name: "updateStillUnadmitted", field: "Phase__c", value: "true", id: "001000000000001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: tc.field, Operation: "isChanged", Value: tc.value}}}, storage.Record{ID: tc.id, Object: "Account"}, definition, frame)
			if supported {
				t.Fatal("unsupported boundary was broadened")
			}
		})
	}
	frame.saveDecisions = false
	_, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "Phase__c", Operation: "isChanged", Value: "true"}}}, storage.Record{Object: "Account"}, definition, frame)
	if supported {
		t.Fatal("non-save decision acquired create handling")
	}
	// Keep the legacy error recognizable instead of swallowing unrelated operators.
	_, _, err := engine.selectFlowStepBranch([]storage.FlowBranch{{Name: "OwnedUnsupported", Criteria: []storage.WorkflowCriteriaItem{{Field: "Phase__c", Operation: "unmodeled", Value: "true"}}}}, storage.Record{Object: "Account"}, definition, newFlowFrame())
	if err == nil || !strings.Contains(err.Error(), "unsupported criteria") {
		t.Fatalf("unrelated error control: %v", err)
	}
}
