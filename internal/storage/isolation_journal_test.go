package storage

import "testing"

func TestIsolationJournalRollbackRebuildsIndexes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName: "Account",
			Indexes: []IndexDefinition{{
				Name:   "Account.Id",
				Object: "Account",
				Fields: []string{"Id"},
			}},
		},
		Records: map[ID]Record{},
	}
	RebuildIndexes(&org)

	journal := NewIsolationJournal(&org)
	mark := journal.Mark()
	id := ID("001000000000001")
	journal.RecordInsert("Account", id)
	account := org.Objects["Account"]
	account.Records[id] = Record{ID: id, Object: "Account"}
	org.Objects["Account"] = account
	RebuildObjectIndexes(&org, "Account")

	if ids, ok := LookupIndex(org.Objects["Account"], "Id", IDValue(id)); !ok || len(ids) != 1 {
		t.Fatalf("pre-rollback index = %#v, %v; want inserted id", ids, ok)
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	if _, exists := org.Objects["Account"].Records[id]; exists {
		t.Fatalf("inserted record survived rollback")
	}
	if ids, ok := LookupIndex(org.Objects["Account"], "Id", IDValue(id)); !ok || len(ids) != 0 {
		t.Fatalf("post-rollback index = %#v, %v; want empty index hit", ids, ok)
	}
}

func TestIsolationJournalNestedMarksRestoreRecordsAndSequences(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account"}, Records: map[ID]Record{}}
	org.IDSequences["Account"] = 10
	journal := NewIsolationJournal(&org)
	outer := journal.Mark()
	id := ID("001000000000001")
	journal.RecordSequence("Account")
	org.IDSequences["Account"] = 11
	journal.RecordInsert("Account", id)
	org.Objects["Account"].Records[id] = Record{ID: id, Object: "Account", Fields: map[string]Value{"Name": StringValue("original")}}
	update := func(name string) {
		before := org.Objects["Account"].Records[id]
		journal.RecordUpdate("Account", id, before)
		after := before.Clone()
		after.Fields["Name"] = StringValue(name)
		org.Objects["Account"].Records[id] = after
	}
	check := func(name string, sequence uint64) {
		t.Helper()
		got := org.Objects["Account"].Records[id]
		if got.Fields["Name"].String != name || org.IDSequences["Account"] != sequence {
			t.Fatalf("rollback state = %#v / %d; want %s / %d", got.Fields, org.IDSequences["Account"], name, sequence)
		}
	}
	inner := journal.Mark()
	update("first")
	update("second")
	journal.RecordSequence("Account")
	org.IDSequences["Account"] = 12
	nested := journal.Mark()
	update("third")
	journal.RecordSequence("Account")
	org.IDSequences["Account"] = 13
	if err := journal.Rollback(nested); err != nil {
		t.Fatal(err)
	}
	check("second", 12)
	update("fourth")
	if err := journal.Rollback(inner); err != nil {
		t.Fatal(err)
	}
	check("original", 11)
	if err := journal.Rollback(outer); err != nil {
		t.Fatal(err)
	}
	if _, exists := org.Objects["Account"].Records[id]; exists {
		t.Fatal("outer rollback resurrected inserted record")
	}
	if org.IDSequences["Account"] != 10 {
		t.Fatalf("outer sequence = %d", org.IDSequences["Account"])
	}
}

func TestIsolationJournalRepeatedUpdatesAcrossMarks(t *testing.T) {
	org := NewOrgState()
	id := ID("001000000000001")
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account"}, Records: map[ID]Record{id: {ID: id, Object: "Account", Fields: map[string]Value{"Name": StringValue("original")}}}}
	journal := NewIsolationJournal(&org)
	outer := journal.Mark()
	update := func(name string) {
		before := org.Objects["Account"].Records[id]
		journal.RecordUpdate("Account", id, before)
		after := before.Clone()
		after.Fields["Name"] = StringValue(name)
		org.Objects["Account"].Records[id] = after
	}
	update("first")
	inner := journal.Mark()
	update("second")
	if err := journal.Rollback(inner); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["Account"].Records[id].Fields["Name"].String; got != "first" {
		t.Fatalf("inner rollback = %q", got)
	}
	if err := journal.Rollback(outer); err != nil {
		t.Fatal(err)
	}
	if got := org.Objects["Account"].Records[id].Fields["Name"].String; got != "original" {
		t.Fatalf("outer rollback = %q", got)
	}
}
