package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A bulk account reassignment of Contacts and Opportunities moves their
// stored Tasks and Events with one scan per DML batch, with the same
// AccountId outcomes as the per-parent refresh.
func TestActivityAccountBatchRefreshScansOnce(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"Account", "Contact", "Opportunity", "Task", "Event"} {
		storage.EnsureStandardObject(&org, name)
	}
	engine := NewEngine(&org)
	engine.DeferAutomation = true
	accounts := engine.Insert([]storage.Record{
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A1")}},
		{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A2")}},
	})
	a1, a2 := accounts[0].ID, accounts[1].ID
	const contacts = 5
	parents := make([]storage.Record, 0, contacts+1)
	for i := 0; i < contacts; i++ {
		parents = append(parents, storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("c"), "AccountId": storage.IDValue(a1)}})
	}
	parents = append(parents, storage.Record{Object: "Opportunity", Fields: map[string]storage.Value{"Name": storage.StringValue("o"), "StageName": storage.StringValue("Prospecting"), "CloseDate": storage.DateValue("2026-01-01"), "AccountId": storage.IDValue(a1)}})
	results := engine.Insert(parents)
	ids := make([]storage.ID, len(results))
	for i, result := range results {
		if !result.Success {
			t.Fatalf("insert parent %d: %#v", i, result)
		}
		ids[i] = result.ID
	}
	opp := ids[contacts]
	activities := []storage.Record{
		// WhatId on the Opportunity wins over the Contact WhoId.
		{Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("t"), "WhatId": storage.IDValue(opp), "WhoId": storage.IDValue(ids[0])}},
		{Object: "Event", Fields: map[string]storage.Value{"Subject": storage.StringValue("e"), "WhoId": storage.IDValue(ids[1]), "DurationInMinutes": storage.IntegerValue(30), "ActivityDateTime": storage.DateTimeValue("2026-01-01T00:00:00Z")}},
	}
	for i := 0; i < contacts; i++ {
		activities = append(activities, storage.Record{Object: "Task", Fields: map[string]storage.Value{"Subject": storage.StringValue("t"), "WhoId": storage.IDValue(ids[i])}})
	}
	for i, result := range engine.Insert(activities) {
		if !result.Success {
			t.Fatalf("insert activity %d: %#v", i, result)
		}
	}
	for _, object := range []string{"Task", "Event"} {
		for id, record := range org.Objects[object].Records {
			if got := idFromStorageValue(record.Fields["AccountId"]); got != a1 {
				t.Fatalf("%s %s AccountId before = %q, want %q", object, id, got, a1)
			}
		}
	}

	updates := make([]storage.Record, 0, len(ids))
	for _, id := range ids[:contacts] {
		updates = append(updates, storage.Record{Object: "Contact", ID: id, Fields: map[string]storage.Value{"AccountId": storage.IDValue(a2)}})
	}
	// The Opportunity keeps A1: its Task must not move even though its WhoId Contact does.
	updates = append(updates, storage.Record{Object: "Opportunity", ID: opp, Fields: map[string]storage.Value{"Amount": storage.DecimalValue("1")}})
	activityRefreshScans = 0
	for i, result := range engine.Update(updates) {
		if !result.Success {
			t.Fatalf("update %d: %#v", i, result)
		}
	}
	if activityRefreshScans != 1 {
		t.Fatalf("activity scans = %d, want 1 for one batch of %d parents", activityRefreshScans, len(updates))
	}
	for _, object := range []string{"Task", "Event"} {
		for id, record := range org.Objects[object].Records {
			want := a2
			if idFromStorageValue(record.Fields["WhatId"]) == opp {
				want = a1
			}
			if got := idFromStorageValue(record.Fields["AccountId"]); got != want {
				t.Fatalf("%s %s AccountId after = %q, want %q", object, id, got, want)
			}
		}
	}

	// A single-record update outside a batch still refreshes immediately.
	activityRefreshScans = 0
	if err := engine.updateOne(storage.Record{Object: "Opportunity", ID: opp, Fields: map[string]storage.Value{"AccountId": storage.IDValue(a2)}}); err != nil {
		t.Fatal(err)
	}
	if activityRefreshScans != 1 {
		t.Fatalf("single update scans = %d, want 1", activityRefreshScans)
	}
	for id, record := range org.Objects["Task"].Records {
		if got := idFromStorageValue(record.Fields["AccountId"]); got != a2 {
			t.Fatalf("Task %s AccountId after opportunity move = %q, want %q", id, got, a2)
		}
	}
}
