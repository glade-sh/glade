package dml

import (
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native change events for cascade-child deletes, merge, and external-ID
// upserts (apextest/testdata/conformance/cdc_g1_g3/org.tsv). The org rows sort
// their event lists, so the engine order is compared sorted.
//   - cascade delete: Account DELETE and Contact DELETE (org.tsv:15,23,39,47).
//   - merge: Account DELETE and survivor Account UPDATE (org.tsv:16,24,40,48).
//   - external-ID upsert: insert then update, coalesced by the VM journal into
//     one CREATE (org.tsv:17,25,41,49).
func TestChangeObserverObservesCascadeMergeAndExternalUpsert(t *testing.T) {
	org := testOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
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
	var events []string
	engine.ChangeObserver = func(operation string, before, after storage.Record) {
		object := after.Object
		if object == "" {
			object = before.Object
		}
		events = append(events, operation+":"+object)
	}
	acct := func(name string) storage.Record {
		return storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
	}
	parent := engine.Insert([]storage.Record{acct("p"), acct("m1"), acct("m2")})
	child := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("c"), "AccountId": storage.IDValue(parent[0].ID)}}})
	if !parent[0].Success || !child[0].Success {
		t.Fatal(parent, child)
	}

	events = nil
	if r := engine.Delete([]storage.Record{{Object: "Account", ID: parent[0].ID}}); !r[0].Success {
		t.Fatal(r)
	}
	if want := []string{"DELETE:Account", "DELETE:Contact"}; !reflect.DeepEqual(sortedEvents(events), want) {
		t.Fatalf("cascade delete events=%v want %v (cdc_g1_g3/org.tsv:15)", events, want)
	}

	events = nil
	if r := engine.Merge(storage.Record{Object: "Account", ID: parent[1].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("merged")}}, []storage.Record{{Object: "Account", ID: parent[2].ID}}); !r[0].Success {
		t.Fatal(r)
	}
	if want := []string{"DELETE:Account", "UPDATE:Account"}; !reflect.DeepEqual(sortedEvents(events), want) {
		t.Fatalf("merge events=%v want %v (cdc_g1_g3/org.tsv:16)", events, want)
	}

	events = nil
	ext := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("x"), "External_Key__c": storage.StringValue("k1")}}
	if r := engine.UpsertWithExternalID([]storage.Record{ext}, "External_Key__c"); !r[0].Success || !r[0].Created {
		t.Fatal(r)
	}
	ext.Fields["Name"] = storage.StringValue("y")
	if r := engine.UpsertWithExternalID([]storage.Record{ext}, "External_Key__c"); !r[0].Success || r[0].Created {
		t.Fatal(r)
	}
	if want := []string{"CREATE:Account", "UPDATE:Account"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("external-ID upsert events=%v want %v (cdc_g1_g3/org.tsv:17)", events, want)
	}
}

func sortedEvents(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
