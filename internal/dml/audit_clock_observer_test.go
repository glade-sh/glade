package dml

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// The host notification and persisted audit fields must use the same statement
// timestamp. Notification must not restore a separate per-write clock sequence.
func TestSystemTimestampObserverUsesStatementClock(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	now := time.Date(2026, 10, 1, 12, 0, 0, 456000000, time.UTC)
	engine.Now = func() time.Time { return now }
	var observed []time.Time
	engine.SystemTimestampObserver = func(stamp time.Time) { observed = append(observed, stamp) }
	for _, name := range []string{"First", "Second"} {
		results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}})
		if len(results) != 1 || !results[0].Success {
			t.Fatalf("insert %s = %#v", name, results)
		}
		stored := org.Objects["Account"].Records[results[0].ID]
		if want := now.Format(time.RFC3339); stored.System.CreatedDate != want || stored.System.LastModifiedDate != want {
			t.Fatalf("generated audit fields = %#v; want statement time %s", stored.System, want)
		}
	}
	if len(observed) != 2 {
		t.Fatalf("observer notifications = %d; want one per statement", len(observed))
	}
	for _, stamp := range observed {
		if !stamp.Equal(now.Truncate(time.Second)) {
			t.Fatalf("reported time = %s; want statement time %s", stamp, now.Truncate(time.Second))
		}
	}
}
