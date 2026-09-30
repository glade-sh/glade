package storage

import (
	"reflect"
	"testing"
)

func TestTaskPriorityPreservesConfiguredRowsAndReset(t *testing.T) {
	org := NewOrgState()
	id := ID("01J000000000099")
	configured := ObjectState{Definition: ObjectDefinition{APIName: "TaskPriority"}, Records: map[ID]Record{id: {ID: id, Object: "TaskPriority", Fields: map[string]Value{"ApiName": StringValue("Urgent"), "IsHighPriority": BooleanValue(true)}}}}
	org.Objects["TaskPriority"] = configured.Clone()
	org.IDSequences["TaskPriority"] = 100
	EnsureDeterministicPlatformData(&org)
	EnsureDeterministicPlatformData(&org)
	ResetNonPlatformData(&org)
	if !reflect.DeepEqual(configured, org.Objects["TaskPriority"]) || org.IDSequences["TaskPriority"] != 100 {
		t.Fatal("configured TaskPriority changed")
	}
}

func TestTaskPriorityUsesConfiguredPicklistAndAvoidsEquivalentIDs(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Task")
	task := org.Objects["Task"]
	task.Definition = task.Definition.Clone()
	field := task.Definition.Fields["Priority"]
	field.PicklistValues = []PicklistValue{{Value: "Normal", Label: "Routine", Active: true}, {Value: "High", Label: "Urgent", Default: true, Active: true}}
	task.Definition.Fields["Priority"] = field
	org.Objects["Task"] = task
	occupied := ID("01J000000000001AAA")
	org.Objects["Owned__c"] = ObjectState{Records: map[ID]Record{occupied: {ID: occupied, Object: "Owned__c"}}}
	ensureTaskPriorityData(&org)
	rows := org.Objects["TaskPriority"].Records
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	for id, row := range rows {
		if IDsEqual(id, occupied) {
			t.Fatal("seed reused occupied identity")
		}
		switch row.Fields["ApiName"].String {
		case "Normal":
			if row.Fields["MasterLabel"].String != "Routine" || row.Fields["IsDefault"].Boolean || row.Fields["IsHighPriority"].Boolean || row.Fields["SortOrder"].Integer != 1 {
				t.Fatal("configured Normal entry changed")
			}
		case "High":
			if row.Fields["MasterLabel"].String != "Urgent" || !row.Fields["IsDefault"].Boolean || !row.Fields["IsHighPriority"].Boolean || row.Fields["SortOrder"].Integer != 2 {
				t.Fatal("configured High entry changed")
			}
		default:
			t.Fatal("unexpected seeded value")
		}
	}
	before := org.Objects["TaskPriority"].Clone()
	sequence := org.IDSequences["TaskPriority"]
	ensureTaskPriorityData(&org)
	ResetNonPlatformData(&org)
	if !reflect.DeepEqual(before, org.Objects["TaskPriority"]) || sequence != org.IDSequences["TaskPriority"] {
		t.Fatal("reapply/reset changed setup rows")
	}
}
