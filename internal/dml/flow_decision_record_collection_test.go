package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce documents Is Empty as a Boolean test of whether a collection is
// empty. Record collections built by Assignment use the same operator as those
// returned by Get Records.
// https://help.salesforce.com/s/articleView?id=flow_ref_operators_condition.htm&language=en_US&type=5
func TestFlowDecisionRecordCollectionIsEmpty(t *testing.T) {
	for _, tc := range []struct {
		name, want  string
		rows        []storage.Record
		initialized bool
		match       bool
	}{
		{name: "uninitializedTrue", want: "true", match: true},
		{name: "uninitializedFalse", want: "false"},
		{name: "emptyTrue", initialized: true, want: "true", match: true},
		{name: "emptyFalse", initialized: true, want: "false"},
		{name: "populatedTrue", initialized: true, rows: []storage.Record{{Object: "Account"}}, want: "true"},
		{name: "populatedFalse", initialized: true, rows: []storage.Record{{Object: "Account"}}, want: "false", match: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			engine := NewEngine(&org)
			frame := newFlowFrame()
			flowCoreSeedScalarDefaults([]storage.FlowVariable{{Name: "PendingRecords", DataType: "SObject", ObjectType: "Account", IsCollection: true}}, frame)
			if tc.initialized {
				frame.collections[flowFrameKey("PendingRecords")] = tc.rows
			}
			branch := storage.FlowBranch{Name: "HasRecords", Criteria: []storage.WorkflowCriteriaItem{{Field: "PendingRecords", Operation: "isEmpty", Value: tc.want}}}
			got, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{Object: "Account"}, org.Objects["Account"].Definition, frame)
			if !supported || got != tc.match {
				t.Fatalf("match=%v supported=%v; want match=%v supported=true", got, supported, tc.match)
			}
		})
	}
}

func TestFlowDecisionRecordCollectionIsEmptyAndControls(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	frame := newFlowFrame()
	frame.collections[flowFrameKey("PendingRecords")] = []storage.Record{{Object: "Account"}}
	definition := org.Objects["Account"].Definition
	source := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Closed")}}
	branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{
		{Field: "PendingRecords", Operation: "isEmpty", Value: "false"},
		{Field: "Name", Operation: "equals", Value: "Open"},
	}}
	if match, supported := engine.evaluateFlowFrameBranch(branch, source, definition, frame); !supported || match {
		t.Fatalf("AND control match=%v supported=%v", match, supported)
	}
	for _, item := range []storage.WorkflowCriteriaItem{
		{Field: "PendingRecords", Operation: "isEmpty", Value: "unknown"},
		{Field: "MissingCollection", Operation: "isEmpty", Value: "true"},
		{Field: "PendingRecords", Operation: "unmodeled", Value: "true"},
	} {
		if _, supported := engine.evaluateFlowFrameBranch(storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{item}}, source, definition, frame); supported {
			t.Fatalf("unsupported boundary broadened: %+v", item)
		}
	}
}

func TestBeforeSaveFlowDecisionAssignedRecordCollection(t *testing.T) {
	for _, tc := range []struct {
		name, want, phase, marker string
		add                       bool
	}{
		{name: "uninitializedIsEmpty", want: "true", phase: "Closed", marker: "matched"},
		{name: "uninitializedIsNotNonempty", want: "false", phase: "Closed", marker: "default"},
		{name: "addedRecordIsNotEmpty", add: true, want: "false", phase: "Closed", marker: "matched"},
		{name: "addedRecordIsNotEmptyTrue", add: true, want: "true", phase: "Closed", marker: "default"},
		{name: "andConditionStillApplies", add: true, want: "false", phase: "Open", marker: "default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			state := org.Objects["Account"]
			state.Definition.Fields["Marker__c"] = storage.Field{APIName: "Marker__c", Type: storage.FieldString}
			steps := []storage.FlowStep{}
			if tc.add {
				steps = append(steps, storage.FlowStep{Kind: "assignment", Assignment: storage.FlowAssignment{Name: "CollectCurrent", Target: "PendingRecords", Operator: "Add", SourceField: "$Record"}})
			}
			mark := func(value string) []storage.FlowStep {
				return []storage.FlowStep{{Kind: "fieldUpdate", FieldUpdates: []storage.WorkflowFieldUpdate{{Field: "Marker__c", LiteralValue: value}}}}
			}
			steps = append(steps, storage.FlowStep{Kind: "decision", Branches: []storage.FlowBranch{
				{Name: "CollectionAndName", Criteria: []storage.WorkflowCriteriaItem{
					{Field: "PendingRecords", Operation: "isEmpty", Value: tc.want},
					{Field: "Name", Operation: "equals", Value: "Closed"},
				}, Steps: mark("matched")},
				{Name: "Default", Default: true, Steps: mark("default")},
			}})
			state.Definition.FlowRules = []storage.FlowRule{{
				Name: "CollectAndDecide", Active: true, TriggerType: "RecordBeforeSave", RecordTriggerType: "Create",
				Variables: []storage.FlowVariable{{Name: "PendingRecords", DataType: "SObject", ObjectType: "Account", IsCollection: true}}, Steps: steps,
			}}
			org.Objects["Account"] = state
			rows := []storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(tc.phase)}}}
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
