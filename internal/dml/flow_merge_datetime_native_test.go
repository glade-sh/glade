package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
	"time"
)

func TestFlowMergeCurrentDateTimeNative(t *testing.T) {
	org := testOrg()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{
		storage.DefaultUserID: {Fields: map[string]storage.Value{
			"LocaleSidKey":   storage.StringValue("en_US"),
			"TimeZoneSidKey": storage.StringValue("America/Los_Angeles"),
		}},
	}}
	engine := NewEngine(&org)
	engine.Now = func() time.Time { return time.Date(2026, 10, 28, 16, 13, 0, 0, time.UTC) }
	got := engine.renderFlowCustomErrorText("datetime={!$Flow.CurrentDateTime}", &flowFrame{})
	if got != "datetime=10/28/2026, 9:13 AM" {
		t.Fatalf("got %q, want native row", got)
	}
}
