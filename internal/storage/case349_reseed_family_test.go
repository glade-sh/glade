package storage

import "testing"

// A RecordType row with an explicitly set family flag is not overwritten.
func TestCase349ReseedRecordTypeFamilyFollowsDefinition(t *testing.T) {
	for _, tc := range []struct {
		name          string
		family, stale bool
	}{{"unflagged", false, true}, {"person", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			id := ID("012000000000101")
			org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}, RecordTypes: []RecordTypeInfo{{ID: id, DeveloperName: "Synthetic", Name: "Synthetic", Active: true, Available: true, PersonAccount: tc.family}}}, Records: map[ID]Record{}}
			org.Objects["RecordType"] = ObjectState{Definition: ObjectDefinition{APIName: "RecordType", KeyPrefix: "012", Fields: map[string]Field{}}, Records: map[ID]Record{id: {ID: id, Object: "RecordType", Fields: map[string]Value{"SobjectType": StringValue("Account"), "DeveloperName": StringValue("Synthetic"), "IsPersonType": BooleanValue(tc.stale), "Description": StringValue("preserve")}}}}
			EnsureDeterministicPlatformData(&org)
			row := org.Objects["RecordType"].Records[id]
			if got := row.Fields["IsPersonType"]; got.Kind != ValueBoolean || got.Boolean != tc.stale {
				t.Errorf("persisted=%v stored=%#v", tc.stale, got)
			}
			if row.Fields["Description"].String != "preserve" {
				t.Errorf("unrelated persisted field changed: %#v", row)
			}
			EnsureDeterministicPlatformData(&org)
			if got := org.Objects["RecordType"].Records[id].Fields["IsPersonType"]; got.Kind != ValueBoolean || got.Boolean != tc.stale {
				t.Errorf("warm persisted=%v stored=%#v", tc.stale, got)
			}
		})
	}
}
