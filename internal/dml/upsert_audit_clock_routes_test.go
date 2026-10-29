package dml_test

import (
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestUpsertAuditClockObserverPreservesPerRowBehavior(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	engine := dml.NewEngine(&org)
	clock := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	engine.Now = func() time.Time {
		clock = clock.Add(time.Second)
		return clock
	}
	var observed []time.Time
	engine.SystemTimestampObserver = func(stamp time.Time) { observed = append(observed, stamp) }
	results := engine.Upsert([]storage.Record{
		{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("First clock row")}},
		{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Second clock row")}},
	})
	if len(results) != 2 || !results[0].Success || !results[1].Success {
		t.Fatalf("upsert: %+v", results)
	}
	if len(observed) != 2 || !observed[1].After(observed[0]) {
		t.Fatalf("per-row audit clock observations=%v", observed)
	}
	for i, result := range results {
		stored := org.Objects["Contact"].Records[result.ID]
		if want := observed[i].Format(time.RFC3339); stored.System.CreatedDate != want || stored.System.LastModifiedDate != want {
			t.Fatalf("row %d audit time=%+v; want %s", i, stored.System, want)
		}
	}
}
