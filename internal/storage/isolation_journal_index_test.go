package storage

import (
	"strconv"
	"testing"
)

// Supplemental coverage exercises span transitions after indexes are populated.
func TestIsolationJournalLargeSpanTransitions(t *testing.T) {
	org := NewOrgState()
	org.Objects["Thing"] = ObjectState{Definition: ObjectDefinition{APIName: "Thing"}, Records: map[ID]Record{}}
	j := NewIsolationJournal(&org)
	outer := j.Mark()
	for i := 0; i < 40; i++ {
		j.RecordInsert("Thing", ID(strconv.Itoa(i)))
	}
	j.RecordUpdate("Thing", "0", Record{ID: "0"})
	if len(j.updated) != 0 {
		t.Fatal("large insert span did not suppress update")
	}
	inner := j.Mark()
	for i := 0; i < 40; i++ {
		id := ID(strconv.Itoa(i))
		j.RecordUpdate("Thing", id, Record{ID: id})
	}
	j.RecordUpdate("Thing", "0", Record{ID: "replacement"})
	if len(j.updated) != 40 || j.updated[0].record.ID != "0" {
		t.Fatal("large update span did not retain first before-image")
	}
	if err := j.Rollback(inner); err != nil {
		t.Fatal(err)
	}
	j.RecordUpdate("Thing", "0", Record{ID: "again"})
	if len(j.updated) != 1 || j.updated[0].record.ID != "again" {
		t.Fatal("rollback retained membership from discarded span")
	}
	if err := j.Rollback(outer); err != nil {
		t.Fatal(err)
	}
}
