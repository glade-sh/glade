package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
	"time"
)

// Native direct-state observations do not establish mixed or malformed states.
func TestR6FlowMixedPermissionAssignmentStateIsUnknown(t *testing.T) {
	now := time.Date(2030, 1, 2, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		fields map[string]storage.Value
	}{
		{"revokedMalformed", map[string]storage.Value{"IsRevoked": storage.BooleanValue(true), "ExpirationDate": storage.DateTimeValue("bad")}},
		{"revokedBoundary", map[string]storage.Value{"IsRevoked": storage.BooleanValue(true), "ExpirationDate": storage.DateTimeValue("2030-01-02T12:00:00Z")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, known := PermissionAssignmentState(storage.Record{Fields: tc.fields}, func() time.Time { return now })
			if known {
				t.Fatal("uncaptured mixed state treated as known")
			}
		})
	}
}
