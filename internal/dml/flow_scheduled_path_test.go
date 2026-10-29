package dml

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// platform_transaction_paths/delivery/api{62,67}/native/org.tsv:3-4:
// F08 runs a past-due zero-minute RecordField path after commit; F09 leaves
// a future path alone. Other schedule shapes stay outside this projection.
func TestFlowScheduledPathEnqueueBoundary(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, field, source, unit string
		offset                    float64
		want                      int
	}{
		{name: "due", field: now.Add(-time.Minute).Format(time.RFC3339), source: "RecordField", unit: "Minutes", want: 1},
		{name: "future", field: now.Add(48 * time.Hour).Format(time.RFC3339), source: "RecordField", unit: "Minutes"},
		{name: "missing", source: "RecordField", unit: "Minutes"},
		{name: "invalid", field: "invalid", source: "RecordField", unit: "Minutes"},
		{name: "offset_unmodeled", field: now.Add(-time.Hour).Format(time.RFC3339), source: "RecordField", unit: "Minutes", offset: 1},
		{name: "source_unmodeled", field: now.Add(-time.Hour).Format(time.RFC3339), source: "RecordTriggerEvent", unit: "Minutes"},
		{name: "unit_unmodeled", field: now.Add(-time.Hour).Format(time.RFC3339), source: "RecordField", unit: "Days"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			id := storage.ID("a00000000000001")
			org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parcel__c"}, Records: map[storage.ID]storage.Record{
				id: {ID: id, Object: "Parcel__c", Fields: map[string]storage.Value{"NextRun__c": storage.DateTimeValue(tc.field)}},
			}}
			engine := NewEngine(&org)
			engine.Now = func() time.Time { return now }
			calls := 0
			engine.FlowPathEnqueuer = func(string, storage.FlowRule, storage.FlowPath, storage.ID) { calls++ }
			engine.enqueueFlowPaths("Parcel__c", id, []storage.FlowRule{{Paths: []storage.FlowPath{{Kind: "scheduled", TimeSource: tc.source, RecordField: "NextRun__c", OffsetUnit: tc.unit, OffsetNumber: tc.offset}}}})
			if calls != tc.want {
				t.Fatalf("enqueued=%d, want %d", calls, tc.want)
			}
		})
	}
}
