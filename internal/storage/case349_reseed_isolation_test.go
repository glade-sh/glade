package storage

import "testing"

// Reconciling synthetic record-type metadata must not mutate a shared snapshot.
func TestCase349ReseedRecordTypeFamilySnapshotIsolation(t *testing.T) {
	for _, route := range []string{"rollback", "template"} {
		for _, family := range []bool{false, true} {
			t.Run(route+map[bool]string{false: "/business", true: "/person"}[family], func(t *testing.T) {
				org := NewOrgState()
				id := ID("012000000000101")
				org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}, RecordTypes: []RecordTypeInfo{{ID: id, DeveloperName: "Synthetic", Active: true, Available: true, PersonAccount: family}}}, Records: map[ID]Record{}}
				EnsureDeterministicPlatformData(&org)
				obj := org.Objects["RecordType"]
				row := obj.Records[id]
				row.Fields["IsPersonType"] = BooleanValue(!family)
				obj.Records[id] = row
				org.Objects["RecordType"] = obj
				var clone OrgState
				if route == "rollback" {
					clone = SnapshotRuntimeOrg(&org)
				} else {
					clone = NewRuntimeTemplate(org).CloneSnapshotOrg()
				}
				EnsureDeterministicPlatformData(&clone)
				if got := clone.Objects["RecordType"].Records[id].Fields["IsPersonType"].Boolean; got != !family {
					t.Errorf("clone family=%v, want %v", got, !family)
				}
				if got := org.Objects["RecordType"].Records[id].Fields["IsPersonType"].Boolean; got != !family {
					t.Errorf("original mutated to %v", got)
				}
			})
		}
	}
}
