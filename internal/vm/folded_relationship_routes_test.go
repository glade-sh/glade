package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFoldedRelationshipConsumerRoutes(t *testing.T) {
	for _, prepared := range []bool{false, true} {
		route := "literal"
		if prepared {
			route = "installed"
		}
		t.Run(route, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["Parcel__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parcel__c", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}}}
			org.Objects["Entry__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
				APIName:   "Entry__c",
				Fields:    map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parcel__c"}, RelationshipName: "Parent__r", ChildRelationshipName: "Entries__r"}},
				Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Parcel__c"}, ParentRelationship: "Parent__r", ChildRelationship: "Entries__r"}},
			}}
			machine := New(nil)
			if prepared {
				machine.SetOrg(&org)
			} else {
				machine.Org = &org
			}
			t.Run("describe", func(t *testing.T) {
				if got := machine.describeChildRelationships("Parcel__c"); len(got) == 0 {
					t.Fatal("missing child describe")
				}
			})
			t.Run("treeSave", func(t *testing.T) {
				child, field, name, ok, err := machine.treeSaveChildRelationship("Parcel__c", "Entries__r")
				if !ok || err != nil || child != "Entry__c" || field != "Parent__c" || name != "Entries__r" {
					t.Fatalf("%s %s %s %v %v", child, field, name, ok, err)
				}
			})
			t.Run("dmlChildField", func(t *testing.T) {
				if !machine.isChildRelationshipField(org.Objects["Parcel__c"].Definition, "Entries__r") {
					t.Fatal("missing DML child field")
				}
			})
			t.Run("queryChildLookup", func(t *testing.T) {
				if got := machine.childRelationshipLookup("Parcel__c", "Entries__r"); got.ChildType != "Entry__c" {
					t.Fatalf("%+v", got)
				}
			})
			t.Run("lazyChildLookup", func(t *testing.T) {
				if got := machine.lazyChildRelationshipLookup("Parcel__c", "Entries__r"); got.ChildType != "Entry__c" {
					t.Fatalf("%+v", got)
				}
			})
			t.Run("loadedChildLookup", func(t *testing.T) {
				if got := machine.loadedChildRelationshipLookup("Parcel__c", "Parcel__c", "Entries__r"); len(got.ChildRelationshipNames) == 0 {
					t.Fatalf("%+v", got)
				}
			})
			t.Run("fieldToken", func(t *testing.T) {
				got, ok := machine.sObjectChildRelationshipNameForFieldToken("Parcel__c", sObjectFieldToken("Entry__c", "Parent__c"))
				if !ok || got != "Entries__r" {
					t.Fatalf("%s %v", got, ok)
				}
			})
			t.Run("jsonChildLookup", func(t *testing.T) {
				got, ok := machine.jsonSObjectChildRelationshipType("Parcel__c", "Entries__r")
				if !ok || got != "List<Entry__c>" {
					t.Fatalf("%s %v", got, ok)
				}
			})
		})
	}
}
