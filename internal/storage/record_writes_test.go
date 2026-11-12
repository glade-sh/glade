package storage

import "testing"

func versionedObject() ObjectState {
	var object ObjectState
	object.SetRecords(make(map[ID]Record))
	return object
}

func TestRecordWritesAdvanceVersion(t *testing.T) {
	object := versionedObject()
	token0, ok := object.RecordsToken()
	if !ok || !token0.Valid() {
		t.Fatal("SetRecords must version the map")
	}
	object.PutRecord("a", Record{ID: "a"})
	token1, _ := object.RecordsToken()
	if token1 == token0 {
		t.Fatal("PutRecord did not advance the version")
	}
	object.DeleteRecord("missing")
	if token2, _ := object.RecordsToken(); token2 != token1 {
		t.Fatal("deleting an absent id must not advance the version")
	}
	object.DeleteRecord("a")
	token3, _ := object.RecordsToken()
	if token3 == token1 || len(object.Records) != 0 {
		t.Fatal("DeleteRecord did not advance the version")
	}
	object.PutRecord("b", Record{ID: "b"})
	object.ClearRecords()
	if token4, _ := object.RecordsToken(); token4 == token3 || len(object.Records) != 0 {
		t.Fatal("ClearRecords did not advance the version")
	}
}

func TestRecordVersionIsSharedByStructCopies(t *testing.T) {
	object := versionedObject()
	stale := object
	object.PutRecord("a", Record{ID: "a"})
	if stale.RecordsVersion() != object.RecordsVersion() {
		t.Fatal("a copy of the struct must see writes made through another copy")
	}
	stale.PutRecord("b", Record{ID: "b"})
	if stale.RecordsVersion() != object.RecordsVersion() || object.RecordsVersion() != 2 {
		t.Fatalf("shared clock out of step: %d %d", stale.RecordsVersion(), object.RecordsVersion())
	}
}

func TestSetRecordsGivesNewMapNewClock(t *testing.T) {
	object := versionedObject()
	before, _ := object.RecordsToken()
	object.SetRecords(make(map[ID]Record))
	after, _ := object.RecordsToken()
	if before == after {
		t.Fatal("a replaced map must not share a token with the old map")
	}
	object.SetRecords(nil)
	if _, ok := object.RecordsToken(); ok {
		t.Fatal("a nil map has no token")
	}
}

func TestUnversionedObjectHasNoToken(t *testing.T) {
	object := ObjectState{Records: map[ID]Record{}}
	if _, ok := object.RecordsToken(); ok {
		t.Fatal("a literal without a clock must be uncacheable")
	}
	object.PutRecord("a", Record{ID: "a"})
	if len(object.Records) != 1 {
		t.Fatal("PutRecord must still write")
	}
}

func TestCloneRuntimeGivesEachCloneItsOwnClock(t *testing.T) {
	org := OrgState{Objects: map[string]ObjectState{}}
	object := versionedObject()
	object.PutRecord("a", Record{ID: "a"})
	org.Objects["Thing__c"] = object

	first := org.CloneRuntime()
	second := org.CloneRuntime()
	f, s := first.Objects["Thing__c"], second.Objects["Thing__c"]
	ft, _ := f.RecordsToken()
	st, _ := s.RecordsToken()
	ot, _ := object.RecordsToken()
	if ft == st || ft == ot || st == ot {
		t.Fatal("clones must not share tokens with each other or the source")
	}
	f.PutRecord("b", Record{ID: "b"})
	if now, _ := s.RecordsToken(); now != st || object.RecordsVersion() != ot.version {
		t.Fatal("a write to one clone moved another clone's version")
	}
}

func TestEnsureMutableObjectRecordsMovesToNewClock(t *testing.T) {
	org := OrgState{Objects: map[string]ObjectState{}}
	object := versionedObject()
	object.PutRecord("a", Record{ID: "a"})
	object.RecordsShared = true
	org.Objects["Thing__c"] = object
	before, _ := object.RecordsToken()
	mutable, cloned := EnsureMutableObjectRecords(&org, "Thing__c")
	if !cloned {
		t.Fatal("shared records must be cloned")
	}
	after, _ := mutable.RecordsToken()
	if before == after {
		t.Fatal("copy-on-write must move to a new clock")
	}
	mutable.PutRecord("b", Record{ID: "b"})
	if now, _ := object.RecordsToken(); now != before {
		t.Fatal("copy-on-write write leaked into the shared map's version")
	}
}

func TestIsolationJournalRollbackAdvancesVersion(t *testing.T) {
	org := OrgState{Objects: map[string]ObjectState{}}
	object := versionedObject()
	object.PutRecord("a", Record{ID: "a", Object: "Thing__c"})
	org.Objects["Thing__c"] = object
	journal := NewIsolationJournal(&org)
	mark := journal.Mark()

	journal.RecordUpdate("Thing__c", "a", object.Records["a"])
	object.PutRecord("a", Record{ID: "a", Object: "Thing__c", Fields: map[string]Value{"Name": StringValue("changed")}})
	journal.RecordInsert("Thing__c", "b")
	object.PutRecord("b", Record{ID: "b", Object: "Thing__c"})
	written, _ := object.RecordsToken()

	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	after, _ := org.Objects["Thing__c"].RecordsToken()
	if after == written {
		t.Fatal("rollback restored rows without advancing the version")
	}
	if _, ok := org.Objects["Thing__c"].Records["b"]; ok {
		t.Fatal("rollback must remove the insert")
	}
	if len(org.Objects["Thing__c"].Records["a"].Fields) != 0 {
		t.Fatal("rollback must restore the update")
	}
}
