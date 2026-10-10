package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Admitted API62 behavior: parent undelete restores only children deleted by
// that parent's cascade, leaving independently deleted siblings in the recycle bin.
func TestUndeleteRestoresOnlyCascadeDeletedChildren(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, CascadeDelete: true}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}}})
	if len(parent) != 1 || !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent)
	}
	children := engine.Insert([]storage.Record{
		{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("cascade"), "AccountId": storage.IDValue(parent[0].ID)}},
		{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("independent"), "AccountId": storage.IDValue(parent[0].ID)}},
	})
	if len(children) != 2 || !children[0].Success || !children[1].Success {
		t.Fatalf("child inserts = %#v", children)
	}
	if deleted := engine.Delete([]storage.Record{{Object: "Contact", ID: children[1].ID}}); len(deleted) != 1 || !deleted[0].Success {
		t.Fatalf("independent delete = %#v", deleted)
	}
	if deleted := engine.Delete([]storage.Record{{Object: "Account", ID: parent[0].ID}}); len(deleted) != 1 || !deleted[0].Success {
		t.Fatalf("parent delete = %#v", deleted)
	}
	if restored := engine.Undelete([]storage.Record{{Object: "Account", ID: parent[0].ID}}); len(restored) != 1 || !restored[0].Success {
		t.Fatalf("parent undelete = %#v", restored)
	}
	if org.Objects["Contact"].Records[children[0].ID].System.IsDeleted {
		t.Fatalf("cascade-deleted child remained deleted: %#v", org.Objects["Contact"].Records[children[0].ID])
	}
	if !org.Objects["Contact"].Records[children[1].ID].System.IsDeleted {
		t.Fatalf("independently deleted child was restored: %#v", org.Objects["Contact"].Records[children[1].ID])
	}
}

func TestCascadeUndeleteJournalRollbackPreservesIndependentDeletion(t *testing.T) {
	org := testOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Contact",
			KeyPrefix: "003",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
			Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, CascadeDelete: true}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}}})
	children := engine.Insert([]storage.Record{
		{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("cascade"), "AccountId": storage.IDValue(parent[0].ID)}},
		{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("independent"), "AccountId": storage.IDValue(parent[0].ID)}},
	})
	if !parent[0].Success || !children[0].Success || !children[1].Success {
		t.Fatalf("setup parent=%#v children=%#v", parent, children)
	}
	if deleted := engine.Delete([]storage.Record{{Object: "Contact", ID: children[1].ID}}); len(deleted) != 1 || !deleted[0].Success {
		t.Fatalf("independent delete = %#v", deleted)
	}

	journal := storage.NewIsolationJournal(&org)
	engine.IsolationJournal = journal
	mark := journal.Mark()
	if deleted := engine.Delete([]storage.Record{{Object: "Account", ID: parent[0].ID}}); len(deleted) != 1 || !deleted[0].Success {
		t.Fatalf("parent delete = %#v", deleted)
	}
	if restored := engine.Undelete([]storage.Record{{Object: "Account", ID: parent[0].ID}}); len(restored) != 1 || !restored[0].Success {
		t.Fatalf("parent undelete = %#v", restored)
	}
	if err := journal.Rollback(mark); err != nil {
		t.Fatal(err)
	}
	if org.Objects["Account"].Records[parent[0].ID].System.IsDeleted {
		t.Fatal("rollback left parent deleted")
	}
	if org.Objects["Contact"].Records[children[0].ID].System.IsDeleted {
		t.Fatal("rollback left cascade child deleted")
	}
	if !org.Objects["Contact"].Records[children[1].ID].System.IsDeleted {
		t.Fatal("rollback restored the independently deleted child")
	}
}

// SF234 exact API62: a junction child deleted through one cascade MasterDetail
// remains deleted when that master is restored while its other cascade master
// stays in the recycle bin.
func TestUndeleteDoesNotRestoreChildWithAnotherDeletedCascadeMaster(t *testing.T) {
	org := testOrg()
	org.Objects["FirstMaster__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "FirstMaster__c", KeyPrefix: "a10", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
	org.Objects["SecondMaster__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "SecondMaster__c", KeyPrefix: "a11", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
	org.Objects["JunctionDetail__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "JunctionDetail__c", KeyPrefix: "a12",
			Fields: map[string]storage.Field{
				"Name":      {APIName: "Name", Type: storage.FieldString},
				"First__c":  {APIName: "First__c", Type: storage.FieldReference, ReferenceTo: []string{"FirstMaster__c"}},
				"Second__c": {APIName: "Second__c", Type: storage.FieldReference, ReferenceTo: []string{"SecondMaster__c"}},
			},
			Relations: []storage.Relationship{
				{Field: "First__c", ParentObjects: []string{"FirstMaster__c"}, CascadeDelete: true},
				{Field: "Second__c", ParentObjects: []string{"SecondMaster__c"}, CascadeDelete: true},
			},
		},
		Records: map[storage.ID]storage.Record{},
	}
	engine := NewEngine(&org)
	first := engine.Insert([]storage.Record{{Object: "FirstMaster__c", Fields: map[string]storage.Value{"Name": storage.StringValue("first")}}})
	second := engine.Insert([]storage.Record{{Object: "SecondMaster__c", Fields: map[string]storage.Value{"Name": storage.StringValue("second")}}})
	if len(first) != 1 || len(second) != 1 || !first[0].Success || !second[0].Success {
		t.Fatalf("master inserts first=%#v second=%#v", first, second)
	}
	child := engine.Insert([]storage.Record{{Object: "JunctionDetail__c", Fields: map[string]storage.Value{
		"Name": storage.StringValue("child"), "First__c": storage.IDValue(first[0].ID), "Second__c": storage.IDValue(second[0].ID),
	}}})
	if len(child) != 1 || !child[0].Success {
		t.Fatalf("child insert = %#v", child)
	}
	if deleted := engine.Delete([]storage.Record{{Object: "FirstMaster__c", ID: first[0].ID}}); len(deleted) != 1 || !deleted[0].Success {
		t.Fatalf("first delete = %#v", deleted)
	}
	if deleted := engine.Delete([]storage.Record{{Object: "SecondMaster__c", ID: second[0].ID}}); len(deleted) != 1 || !deleted[0].Success {
		t.Fatalf("second delete = %#v", deleted)
	}
	if restored := engine.Undelete([]storage.Record{{Object: "FirstMaster__c", ID: first[0].ID}}); len(restored) != 1 || !restored[0].Success {
		t.Fatalf("first undelete = %#v", restored)
	}
	stored := org.Objects["JunctionDetail__c"].Records[child[0].ID]
	if !stored.System.IsDeleted {
		t.Fatalf("child restored while second cascade master remains deleted: %#v", stored)
	}
	if !storage.IDsEqual(idFromStorageValue(stored.Fields["First__c"]), first[0].ID) || !storage.IDsEqual(idFromStorageValue(stored.Fields["Second__c"]), second[0].ID) {
		t.Fatalf("child relationship identities changed: %#v", stored)
	}
}
