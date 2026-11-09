package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
	"time"
)

// Native rows: r6flow_expiry/org.tsv future, past, revoked.
func TestR6FlowFeaturePermissionAssignmentState(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, expiry  string
		revoked, want bool
	}{{"future", "2030-01-04T12:00:00Z", false, true}, {"past", "2029-12-31T12:00:00Z", false, false}, {"revoked", "", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))}}}}
			fields := map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")}
			if tc.expiry != "" {
				fields["ExpirationDate"] = storage.DateTimeValue(tc.expiry)
			}
			if tc.revoked {
				fields["IsRevoked"] = storage.BooleanValue(true)
			}
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: fields}}}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.executionUser = Object("User")
			machine.executionUser.Fields["Id"] = String(string(storage.DefaultUserID))
			machine.fakeNow = now
			if got := machine.currentUserHasPermission("R6_Bypass"); got != tc.want {
				t.Fatalf("permission=%v want=%v", got, tc.want)
			}
		})
	}
}

// The expiry capture covers direct assignments only; other states preserve the existing lookup.
func TestR6FlowFeaturePermissionUncapturedStatePreservesLookup(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields map[string]storage.Value
	}{
		{"malformed", map[string]storage.Value{"ExpirationDate": storage.DateTimeValue("bad")}},
		{"group", map[string]storage.Value{"PermissionSetGroupId": storage.IDValue("0PG000000000001"), "IsRevoked": storage.BooleanValue(true)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))}}}}
			fields := map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")}
			for key, value := range tc.fields {
				fields[key] = value
			}
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: fields}}}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.executionUser = Object("User")
			machine.executionUser.Fields["Id"] = String(string(storage.DefaultUserID))
			if !machine.currentUserHasPermission("R6_Bypass") {
				t.Fatal("uncaptured assignment lost existing grant")
			}
		})
	}
}
