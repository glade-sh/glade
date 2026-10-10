package dml

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestLoadedReferenceInputIsUpdateOnlyAndNeverStored(t *testing.T) {
	org := testOrg()
	parentID := storage.ID("001000000000001")
	childID := storage.ID("003000000000001")
	account := org.Objects["Account"]
	account.Records[parentID] = storage.Record{ID: parentID, Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("deleted parent")}, System: storage.SystemFields{IsDeleted: true}}
	org.Objects["Account"] = account
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact", KeyPrefix: "003", Fields: map[string]storage.Field{
		"LastName": {APIName: "LastName", Type: storage.FieldString}, "ParentId": {APIName: "ParentId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
	}, Relations: []storage.Relationship{{Field: "ParentId", ParentObjects: []string{"Account"}, SetNullOnDelete: true}}}, Records: map[storage.ID]storage.Record{childID: {ID: childID, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("child")}, ExplicitNulls: map[string]bool{"ParentId": true}}}}
	engine := NewEngine(&org)
	input := storage.Record{ID: childID, Object: "Contact", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parentID)}}
	// A direct Engine caller without provenance remains strict.
	assertDeleted := func(result Result) {
		t.Helper()
		if result.Success || result.StatusCode != "ENTITY_IS_DELETED" || result.Error != "entity is deleted" || len(result.Fields) != 0 {
			t.Fatalf("deleted reference result: %#v", result)
		}
	}
	assertDeleted(engine.Update([]storage.Record{input})[0])
	loaded := input.Clone()
	loaded.LoadedReferences = map[string]storage.ID{"ParentId": parentID}
	if result := engine.Update([]storage.Record{loaded})[0]; !result.Success {
		t.Fatal(result)
	}
	stored := org.Objects["Contact"].Records[childID]
	if value, _ := stored.GetField("ParentId"); !storage.IDsEqual(value.ID, parentID) || stored.HasExplicitNull("ParentId") {
		t.Fatalf("selected value not restored: %#v", stored)
	}
	if len(stored.LoadedReferences) != 0 {
		t.Fatal("input provenance was persisted")
	}
	assertDeleted(engine.Update([]storage.Record{input})[0])
	inserted := loaded.Clone()
	inserted.ID = ""
	assertDeleted(engine.Insert([]storage.Record{inserted})[0])
	raw, err := json.Marshal(loaded)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip storage.Record
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if len(roundTrip.LoadedReferences) != 0 {
		t.Fatal("input provenance survived serialization")
	}
	assertDeleted(engine.Update([]storage.Record{roundTrip})[0])
	copied := loaded.Clone()
	copied.LoadedReferences["ParentId"] = "001000000000002"
	if loaded.LoadedReferences["ParentId"] != parentID {
		t.Fatal("input clone aliases provenance")
	}
	assertDeleted(engine.Update([]storage.Record{copied})[0])
	// Removing the actual target models an absent/hard-deleted record. An old
	// input capability must never bypass that missing-target check.
	delete(org.Objects["Account"].Records, parentID)
	result := engine.Update([]storage.Record{loaded})[0]
	if result.Success || result.StatusCode != "FIELD_INTEGRITY_EXCEPTION" {
		t.Fatalf("missing target accepted: %#v", result)
	}
}
