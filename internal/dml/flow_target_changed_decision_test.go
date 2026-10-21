package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Compose shared Flow targets with the documented direct-field Is Changed
// comparison. Creation has no prior value; updates compare the stored row.
// https://help.salesforce.com/s/articleView?id=flow_ref_operators_condition.htm&type=5
func TestFlowTargetChangedDecisionUsesStoredPrior(t *testing.T) {
	for _, tc := range []struct {
		name, prior, current, operand, want string
	}{
		{"creation", "", "Ready", "true", "Other"},
		{"creationFalse", "", "Ready", "false", "Selected"},
		{"changed", "Draft", "Ready", "true", "Selected"},
		{"unchanged", "Ready", "Ready", "true", "Other"},
		{"changedFalse", "Draft", "Ready", "false", "Other"},
		{"unchangedFalse", "Ready", "Ready", "false", "Selected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			state := org.Objects["Account"]
			state.Definition.Fields["Phase__c"] = storage.Field{APIName: "Phase__c", Type: storage.FieldString}
			state.Definition.Fields["Outcome__c"] = storage.Field{APIName: "Outcome__c", Type: storage.FieldString}
			state.Definition.FlowRules = []storage.FlowRule{{
				Name: "SharedChangeDecision", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "CreateAndUpdate",
				Steps: []storage.FlowStep{{Kind: "reference", Target: "route"}},
				StepTargets: map[string][]storage.FlowStep{"route": {{Kind: "decision", Branches: []storage.FlowBranch{
					{Name: "Changed", Criteria: []storage.WorkflowCriteriaItem{{Field: "Phase__c", Operation: "isChanged", Value: tc.operand}}, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Selected"}}}}},
					{Name: "Other", Default: true, Steps: []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Outcome__c", LiteralValue: "Other"}}}}},
				}}}},
			}}
			current := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout"), "Phase__c": storage.StringValue(tc.current)}}
			if tc.prior != "" {
				current.ID = "001000000000001"
				state.Records[current.ID] = storage.Record{Object: "Account", ID: current.ID, Fields: map[string]storage.Value{"Name": storage.StringValue("Sprout"), "Phase__c": storage.StringValue(tc.prior)}}
			}
			org.Objects["Account"] = state
			rows := []storage.Record{current}
			engine := NewEngine(&org)
			if err := engine.ApplyBeforeSaveFlows(rows); err != nil {
				t.Fatal(err)
			}
			if got := rows[0].Fields["Outcome__c"].String; got != tc.want {
				t.Fatalf("outcome=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestFlowChangedDecisionKeepsDirectLiteralBoundary(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	definition := org.Objects["Account"].Definition
	frame := newFlowFrame()
	frame.saveDecisions = true
	frame.recordCreations = engine.RecordCreations
	frame.records[flowFrameKey("$Record__Prior")] = storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Before")}}
	for _, tc := range []struct{ name, field, source, value string }{
		{"sourceOperand", "Name", "flag", "true"},
		{"relationship", "Parent.Name", "", "true"},
		{"invalidField", "Missing__c", "", "true"},
		{"nonBoolean", "Name", "", "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: tc.field, SourceField: tc.source, Operation: "isChanged", Value: tc.value}}}, storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("After")}}, definition, frame)
			if supported {
				t.Fatal("indirect or invalid Is Changed criterion was admitted")
			}
		})
	}
}
