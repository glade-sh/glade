package storage

import "testing"

// Membership is scoped to the current mark and uses raw object and ID keys.
func TestIsolationJournalMembershipAcrossRollback(t *testing.T) {
	org := NewOrgState()
	org.Objects["Thing"] = ObjectState{Definition: ObjectDefinition{APIName: "Thing"}, Records: map[ID]Record{}}
	org.Objects["thing"] = ObjectState{Definition: ObjectDefinition{APIName: "thing"}, Records: map[ID]Record{}}
	j := NewIsolationJournal(&org)
	outer := j.Mark()
	j.RecordInsert("Thing", "a")
	j.RecordInsert("Thing", "a")
	j.RecordUpdate("Thing", "a", Record{})
	if len(j.updated) != 0 {
		t.Fatal("update of inserted key recorded a before-image")
	}
	inner := j.Mark()
	j.RecordUpdate("Thing", "a", Record{ID: "a"})
	j.RecordUpdate("Thing", "a", Record{ID: "different"})
	j.RecordUpdate("thing", "a", Record{ID: "a"})
	j.RecordUpdate("Thing", "A", Record{ID: "A"})
	if len(j.updated) != 3 {
		t.Fatalf("before-images = %d, want 3 distinct raw keys", len(j.updated))
	}
	if err := j.Rollback(inner); err != nil {
		t.Fatal(err)
	}
	j.RecordUpdate("Thing", "a", Record{ID: "again"})
	if len(j.updated) != 1 || j.updated[0].record.ID != "again" {
		t.Fatal("rollback did not permit a fresh before-image")
	}
	if err := j.Rollback(outer); err != nil {
		t.Fatal(err)
	}
}

func TestIsolationJournalFailedRollbackRetainsMembership(t *testing.T) {
	org := NewOrgState()
	j := NewIsolationJournal(&org)
	mark := j.Mark()
	j.RecordUpdate("Missing", "a", Record{ID: "first"})
	if err := j.Rollback(mark); err == nil {
		t.Fatal("rollback of missing object succeeded")
	}
	j.RecordUpdate("Missing", "a", Record{ID: "second"})
	if len(j.updated) != 1 || j.updated[0].record.ID != "first" {
		t.Fatal("failed rollback lost the original before-image")
	}
}
