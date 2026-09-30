package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestChildRelationshipIDWidthsWithAndWithoutStorageIndex(t *testing.T) {
	for _, indexed := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("indexed=%t/reverse=%t", indexed, reverse), func(t *testing.T) {
				parentID, foreignID := storage.ID("001000000000001"), storage.ID("001000000000001AAA")
				if reverse {
					parentID, foreignID = foreignID, parentID
				}
				org := storage.NewOrgState()
				org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: map[storage.ID]storage.Record{parentID: {ID: parentID, Object: "Account"}}}
				child := storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact",
					Fields:    map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}},
					Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}},
				}, Records: map[storage.ID]storage.Record{
					"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(parentID)}},
				}}
				if indexed {
					child.Definition.Indexes = []storage.IndexDefinition{{Name: "Contact.Account", Object: "Contact", Fields: []string{"AccountId"}}}
				}
				org.Objects["Contact"] = child
				storage.RebuildIndexes(&org)
				query, err := Parse("SELECT Id,(SELECT Id,AccountId FROM Contacts) FROM Account")
				if err != nil {
					t.Fatal(err)
				}
				cache := NewExecutionCache()
				first, err := ExecuteWithCache(org, query, cache)
				if err != nil {
					t.Fatal(err)
				}
				if len(first.Records) != 1 || len(first.Records[0].Children["Contacts"]) != 1 {
					t.Fatalf("warm query: %#v", first)
				}
				child = org.Objects["Contact"]
				child.Records["003000000000002"] = storage.Record{ID: "003000000000002", Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(foreignID)}}
				org.Objects["Contact"] = child
				storage.RebuildObjectIndexes(&org, "Contact")
				after, err := ExecuteWithCache(org, query, cache)
				if err != nil {
					t.Fatal(err)
				}
				if len(after.Records) != 1 {
					t.Fatalf("parent population: %#v", after)
				}
				rows := after.Records[0].Children["Contacts"]
				if len(rows) != 2 || rows[0].ID != "003000000000001" || rows[1].ID != "003000000000002" {
					t.Fatalf("child population: %#v", rows)
				}
			})
		}
	}
}
