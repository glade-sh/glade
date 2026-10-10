package storage

import (
	"reflect"
	"testing"
)

func TestTaskStatusDefaultPopulationAndIdentity(t *testing.T) {
	org := NewOrgState()
	// The first four 01J IDs belong to the existing LeadStatus seed. Reserve the
	// next one in a different object using its equivalent 18-character form.
	occupied := ID("01J000000000005AAA")
	org.Objects["Owned__c"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Owned__c"},
		Records:    map[ID]Record{occupied: {ID: occupied, Object: "Owned__c"}},
	}
	EnsureDeterministicPlatformData(&org)
	statuses := org.Objects["TaskStatus"]
	if len(statuses.Records) != 5 {
		t.Fatalf("TaskStatus count = %d, want 5", len(statuses.Records))
	}
	want := []struct {
		label     string
		closed    bool
		isDefault bool
	}{
		{"Not Started", false, true},
		{"In Progress", false, false},
		{"Completed", true, false},
		{"Waiting on someone else", false, false},
		{"Deferred", false, false},
	}
	seen := make(map[int64]bool)
	for id, row := range statuses.Records {
		order := row.Fields["SortOrder"]
		if order.Kind != ValueInteger || order.Integer < 1 || order.Integer > 5 || seen[order.Integer] {
			t.Fatalf("invalid/duplicate SortOrder: %#v", order)
		}
		seen[order.Integer] = true
		expected := want[order.Integer-1]
		for field, value := range map[string]Value{
			"MasterLabel": StringValue(expected.label), "ApiName": StringValue(expected.label),
			"IsClosed": BooleanValue(expected.closed), "IsDefault": BooleanValue(expected.isDefault),
		} {
			if !reflect.DeepEqual(row.Fields[field], value) {
				t.Fatalf("%s %s = %#v, want %#v", expected.label, field, row.Fields[field], value)
			}
		}
		if row.ID != id || row.Object != "TaskStatus" {
			t.Fatalf("incorrect row identity: %#v", row)
		}
		for name, object := range org.Objects {
			if name == "TaskStatus" {
				continue
			}
			if _, _, found := LookupRecordByID(object.Records, id); found {
				t.Fatalf("TaskStatus %s collides with %s", id, name)
			}
		}
	}
	if org.IDSequences["TaskStatus"] != 10 {
		t.Fatalf("TaskStatus sequence = %d, want 10 after skipped IDs", org.IDSequences["TaskStatus"])
	}
	before := statuses.Clone()
	EnsureDeterministicPlatformData(&org)
	if !reflect.DeepEqual(org.Objects["TaskStatus"], before) || org.IDSequences["TaskStatus"] != 10 {
		t.Fatal("repeated initialization changed TaskStatus")
	}
	ResetNonPlatformData(&org)
	if !reflect.DeepEqual(org.Objects["TaskStatus"], before) || org.IDSequences["TaskStatus"] != 10 {
		t.Fatal("non-platform reset changed TaskStatus")
	}
	if len(org.Objects["Owned__c"].Records) != 0 {
		t.Fatal("non-platform reset did not clear ordinary records")
	}
}

func TestTaskStatusPreservesConfiguredPopulation(t *testing.T) {
	org := NewOrgState()
	id := ID("01J000000000099")
	configured := ObjectState{
		Definition: ObjectDefinition{APIName: "TaskStatus", KeyPrefix: "01J", Fields: map[string]Field{"CustomFlag": {APIName: "CustomFlag", Type: FieldBoolean}}},
		Records: map[ID]Record{id: {ID: id, Object: "TaskStatus", Fields: map[string]Value{
			"MasterLabel": StringValue("Owned finished"), "ApiName": StringValue("OwnedFinished"),
			"IsClosed": BooleanValue(true), "IsDefault": BooleanValue(true),
			"SortOrder": IntegerValue(42), "CustomFlag": BooleanValue(true),
		}}},
	}
	org.Objects["TaskStatus"] = configured.Clone()
	org.IDSequences["TaskStatus"] = 100
	EnsureDeterministicPlatformData(&org)
	EnsureDeterministicPlatformData(&org)
	ResetNonPlatformData(&org)
	if !reflect.DeepEqual(org.Objects["TaskStatus"], configured) || org.IDSequences["TaskStatus"] != 100 {
		t.Fatal("configured TaskStatus table, schema or sequence changed")
	}
}

func TestTaskStatusEmptyPopulationHonorsSequence(t *testing.T) {
	org := NewOrgState()
	org.IDSequences["TaskStatus"] = 20
	EnsureDeterministicPlatformData(&org)
	if org.IDSequences["TaskStatus"] != 25 {
		t.Fatalf("TaskStatus sequence = %d, want 25", org.IDSequences["TaskStatus"])
	}
	for n := uint64(21); n <= 25; n++ {
		id := ID("01J" + leftPadBase36(n, 12))
		if _, found := org.Objects["TaskStatus"].Records[id]; !found {
			t.Fatalf("missing expected TaskStatus ID %s", id)
		}
	}
}
