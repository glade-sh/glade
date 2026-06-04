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
