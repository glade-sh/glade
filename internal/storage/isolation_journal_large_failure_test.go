package storage

import (
	"strconv"
	"testing"
)

// A failed rollback must leave a populated membership index in sync with the journal.
func TestIsolationJournalLargeSpanFailedRollback(t *testing.T) {
	org := NewOrgState()
	j := NewIsolationJournal(&org)
	mark := j.Mark()
	for i := 0; i < 40; i++ {
		id := ID(strconv.Itoa(i))
		j.RecordUpdate("Missing", id, Record{ID: id})
	}
	if err := j.Rollback(mark); err == nil {
		t.Fatal("rollback of missing object succeeded")
	}
	j.RecordUpdate("Missing", "0", Record{ID: "replacement"})
	if len(j.updated) != 40 || j.updated[0].record.ID != "0" {
		t.Fatal("failed rollback lost indexed membership or the first before-image")
	}
}
