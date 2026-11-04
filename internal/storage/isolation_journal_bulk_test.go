package storage

import (
	"strconv"
	"testing"
)

// Exercise distinct keys and repeated keys in one span, then roll back.
func TestIsolationJournalBulkMembership(t *testing.T) {
	org := NewOrgState()
	org.Objects["Thing"] = ObjectState{Definition: ObjectDefinition{APIName: "Thing"}, Records: map[ID]Record{}}
	j := NewIsolationJournal(&org)
	mark := j.Mark()
	const count = 3000
	for i := 0; i < count; i++ {
		id := ID(strconv.Itoa(i))
		j.RecordUpdate("Thing", id, Record{ID: id})
	}
	for i := 0; i < count; i++ {
		id := ID(strconv.Itoa(i))
		j.RecordUpdate("Thing", id, Record{ID: id})
	}
	if len(j.updated) != count {
		t.Fatalf("before-images = %d, want %d", len(j.updated), count)
	}
	if err := j.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	if len(org.Objects["Thing"].Records) != count || len(j.updated) != 0 {
		t.Fatal("bulk rollback did not restore all before-images")
	}
}
