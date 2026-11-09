package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
	"time"
)

// Native direct assignment state: testdata/r6flow_assignment_active/org.tsv:2-13.
func TestR6FlowAssignmentInsertActiveDefault(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, expiry string
		want         bool
	}{{"future", "2030-01-04T12:00:00Z", true}, {"past", "2029-12-31T12:00:00Z", false}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			engine := NewEngine(&org)
			engine.Now = func() time.Time { return now }
			rec := storage.Record{Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"PermissionSetId": storage.IDValue("0PS000000000001"), "ExpirationDate": storage.DateTimeValue(tc.expiry), "IsActive": storage.BooleanValue(false)}}
			engine.applyPermissionSetAssignmentDefaults(rec.Object, &rec)
			if got := rec.Fields["IsActive"]; got.Kind != storage.ValueBoolean || got.Boolean != tc.want {
				t.Fatalf("IsActive=%v want=%v", got, tc.want)
			}
		})
	}
}
