package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// RecalculatePermissionSetGroups writes the aggregate PermissionSet row and
// the group status while the engine org's rows can be shared with a
// rollback snapshot, as WithTransaction and beginRollbackPoint leave them.
// Both writes must clone through EnsureMutableObjectRecords so the
// snapshot keeps its original rows.
func TestPermissionSetGroupRecalculationLeavesRollbackSnapshotUnchanged(t *testing.T) {
	const (
		groupID     storage.ID = "0PG000000000001AAA"
		aggregateID storage.ID = "0PS000000000001AAA"
		memberID    storage.ID = "0PS000000000002AAA"
	)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "PermissionSet")
	sets := org.Objects["PermissionSet"]
	sets.SetRecords(map[storage.ID]storage.Record{
		aggregateID: {ID: aggregateID, Object: "PermissionSet", Fields: map[string]storage.Value{
			"Type": storage.StringValue("Group"), "PermissionSetGroupId": storage.IDValue(groupID),
		}},
		memberID: {ID: memberID, Object: "PermissionSet", Fields: map[string]storage.Value{
			"PermissionsApiEnabled": storage.BooleanValue(true),
		}},
	})
	org.Objects["PermissionSet"] = sets
	storage.EnsureStandardObject(&org, "PermissionSetGroup")
	groups := org.Objects["PermissionSetGroup"]
	groups.SetRecords(map[storage.ID]storage.Record{
		groupID: {ID: groupID, Object: "PermissionSetGroup", Fields: map[string]storage.Value{
			"DeveloperName": storage.StringValue("Recalc"), "MasterLabel": storage.StringValue("Recalc"),
			"Status": storage.StringValue("Outdated"),
		}},
	})
	org.Objects["PermissionSetGroup"] = groups
	storage.EnsureStandardObject(&org, "PermissionSetGroupComponent")
	components := org.Objects["PermissionSetGroupComponent"]
	components.SetRecords(map[storage.ID]storage.Record{
		"01Z000000000001AAA": {ID: "01Z000000000001AAA", Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{
			"PermissionSetGroupId": storage.IDValue(groupID), "PermissionSetId": storage.IDValue(memberID),
		}},
	})
	org.Objects["PermissionSetGroupComponent"] = components

	engine := NewEngine(&org)
	snapshot := storage.SnapshotRuntimeOrg(&org)
	if !org.Objects["PermissionSet"].RecordsShared || !org.Objects["PermissionSetGroup"].RecordsShared {
		t.Fatal("rows are not shared with the rollback snapshot")
	}
	if err := engine.RecalculatePermissionSetGroups([]storage.ID{groupID}, nil); err != nil {
		t.Fatal(err)
	}

	if got := org.Objects["PermissionSet"].Records[aggregateID].Fields["PermissionsApiEnabled"].Boolean; !got {
		t.Fatal("recalculation did not update the aggregate permissions")
	}
	if got := org.Objects["PermissionSetGroup"].Records[groupID].Fields["Status"].String; got != "Updated" {
		t.Fatalf("group status = %q, want Updated", got)
	}
	if got := snapshot.Objects["PermissionSet"].Records[aggregateID].Fields["PermissionsApiEnabled"].Boolean; got {
		t.Fatal("recalculation leaked into the rollback snapshot's shared rows")
	}
	if got := snapshot.Objects["PermissionSetGroup"].Records[groupID].Fields["Status"].String; got != "Outdated" {
		t.Fatalf("rollback snapshot group status = %q, want Outdated", got)
	}
}
