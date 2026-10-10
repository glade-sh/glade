package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestManualDuplicateRecordSetCount(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"Contact", "DuplicateRule", "DuplicateRecordSet", "DuplicateRecordItem"} {
		storage.EnsureStandardObject(&org, name)
	}
	// Explicit context isolates the count regression from default platform seeding.
	rules := org.Objects["DuplicateRule"]
	ruleID := storage.ID("0Bm000000000099")
	rules.Records[ruleID] = storage.Record{ID: ruleID, Object: "DuplicateRule", Fields: map[string]storage.Value{"SobjectType": storage.StringValue("Contact")}}
	org.Objects["DuplicateRule"] = rules
	engine := NewEngine(&org)
	require := func(results []Result) {
		t.Helper()
		for _, r := range results {
			if !r.Success {
				t.Fatalf("DML failed: %#v", results)
			}
		}
	}
	contacts := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("First")}}, {Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Second")}}})
	require(contacts)
	sets := engine.Insert([]storage.Record{{Object: "DuplicateRecordSet", Fields: map[string]storage.Value{"DuplicateRuleId": storage.IDValue(ruleID)}}})
	require(sets)
	items := engine.Insert([]storage.Record{{Object: "DuplicateRecordItem", Fields: map[string]storage.Value{"DuplicateRecordSetId": storage.IDValue(sets[0].ID), "RecordId": storage.IDValue(contacts[0].ID)}}, {Object: "DuplicateRecordItem", Fields: map[string]storage.Value{"DuplicateRecordSetId": storage.IDValue(sets[0].ID), "RecordId": storage.IDValue(contacts[1].ID)}}})
	require(items)
	assertCount := func(want int64) {
		t.Helper()
		count := org.Objects["DuplicateRecordSet"].Records[sets[0].ID].Fields["RecordCount"]
		if count.Kind != storage.ValueInteger || count.Integer != want {
			t.Fatalf("RecordCount=%#v; want %d", count, want)
		}
	}
	assertCount(2)
	for i, item := range items {
		record := org.Objects["DuplicateRecordItem"].Records[item.ID]
		if !storage.IDsEqual(idFromStorageValue(record.Fields["RecordId"]), contacts[i].ID) {
			t.Fatal("item lost referenced Contact")
		}
	}
	// Local-only lifecycle controls: normal deletion and journal rollback.
	engine.IsolationJournal = storage.NewIsolationJournal(&org)
	mark := engine.IsolationJournal.Mark()
	require(engine.Delete([]storage.Record{{Object: "DuplicateRecordItem", ID: items[0].ID}}))
	assertCount(1)
	if err := engine.IsolationJournal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	assertCount(2)
	if org.Objects["DuplicateRecordItem"].Records[items[0].ID].System.IsDeleted {
		t.Fatal("rollback did not restore item")
	}
}
