package dml

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDeleteSetNullPreservesSnapshotAndRollback(t *testing.T) {
	for _, withContext := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "batch"}[withContext], func(t *testing.T) {
			org := testOrg()
			org.Objects["Borrower__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Borrower__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
				"Parent__c":      {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
				"Unspecified__c": {APIName: "Unspecified__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			}, Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Account"}, SetNullOnDelete: true}, {Field: "Unspecified__c", ParentObjects: []string{"Account"}}}}, Records: map[storage.ID]storage.Record{}}
			// Stored relationship metadata survives the same JSON representation used by org snapshots.
			encoded, err := json.Marshal(org.Objects["Borrower__c"].Definition)
			if err != nil {
				t.Fatal(err)
			}
			var definition storage.ObjectDefinition
			if err = json.Unmarshal(encoded, &definition); err != nil {
				t.Fatal(err)
			}
			if !definition.Relations[0].SetNullOnDelete || definition.Relations[1].SetNullOnDelete {
				t.Fatal("relationship metadata changed")
			}
			engine := NewEngine(&org)
			parents := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("first")}}, {Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("second")}}})
			for _, r := range parents {
				if !r.Success {
					t.Fatal(r)
				}
			}
			// Reference normalization must treat15/18-character forms as the same existing ID.
			ref := storage.ID(string(parents[0].ID) + "AAA")
			children := engine.Insert([]storage.Record{{Object: "Borrower__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(ref), "Unspecified__c": storage.IDValue(ref)}}, {Object: "Borrower__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parents[1].ID)}}})
			for _, r := range children {
				if !r.Success {
					t.Fatal(r)
				}
			}
			before := org.Objects["Borrower__c"].Records[children[0].ID].Clone()
			snapshot := storage.SnapshotRuntimeOrg(&org)
			engine.IsolationJournal = storage.NewIsolationJournal(&org)
			mark := engine.IsolationJournal.Mark()
			input := storage.Record{Object: "Account", ID: parents[0].ID}
			if withContext {
				if result := engine.Delete([]storage.Record{input})[0]; !result.Success {
					t.Fatal(result)
				}
			} else if err := engine.deleteOne(input); err != nil {
				t.Fatal(err)
			}
			changed := org.Objects["Borrower__c"].Records[children[0].ID]
			if _, present := changed.GetField("Parent__c"); present || !changed.HasExplicitNull("Parent__c") {
				t.Fatalf("lookup not cleared: %#v", changed)
			}
			if v, _ := changed.GetField("Unspecified__c"); !storage.IDsEqual(v.ID, ref) {
				t.Fatal("unspecified relationship was changed")
			}
			if v, _ := org.Objects["Borrower__c"].Records[children[1].ID].GetField("Parent__c"); v.ID != parents[1].ID {
				t.Fatal("unrelated parent reference changed")
			}
			if v, _ := snapshot.Objects["Borrower__c"].Records[children[0].ID].GetField("Parent__c"); v.ID != ref {
				t.Fatal("shared snapshot mutated")
			}
			if v, _ := before.GetField("Parent__c"); v.ID != ref {
				t.Fatal("previous record view mutated")
			}
			if err := engine.IsolationJournal.Rollback(mark); err != nil {
				t.Fatal(err)
			}
			restored := org.Objects["Borrower__c"].Records[children[0].ID]
			if v, _ := restored.GetField("Parent__c"); v.ID != ref || restored.HasExplicitNull("Parent__c") {
				t.Fatal("rollback did not restore lookup")
			}
			if org.Objects["Account"].Records[parents[0].ID].System.IsDeleted {
				t.Fatal("rollback did not restore parent")
			}
		})
	}
}
