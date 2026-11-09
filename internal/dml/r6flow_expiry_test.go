package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
	"time"
)

// API 62/67 captures: r6flow_expiry/org.tsv and r6flow_unrelated/org.tsv.
func TestR6FlowPermissionAssignmentState(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, expiration                    string
		revoked, unrelated, want, supported bool
	}{
		{"future", "2030-01-04T12:00:00Z", false, false, true, true},
		{"past", "2029-12-31T12:00:00Z", false, false, false, true},
		{"revoked", "", true, false, false, true},
		{"unrelatedFuture", "2030-01-04T12:00:00Z", false, true, false, true},
		{"exactBoundary", "2030-01-02T12:00:00Z", false, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			grant := "R6_Bypass"
			if tc.unrelated {
				grant = "R6_Other"
			}
			org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue(grant))}}}}
			fields := map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")}
			if tc.expiration != "" {
				fields["ExpirationDate"] = storage.DateTimeValue(tc.expiration)
			}
			if tc.revoked {
				fields["IsRevoked"] = storage.BooleanValue(true)
			}
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: fields}}}
			engine := NewEngine(&org)
			engine.Now = func() time.Time { return now }
			match, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "true"})
			if !handled || supported != tc.supported || (supported && match != tc.want) {
				t.Fatalf("match=%v supported=%v handled=%v", match, supported, handled)
			}
		})
	}
}
