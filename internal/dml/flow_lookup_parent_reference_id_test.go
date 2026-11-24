package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// These schema, absence, and deletion controls preserve the preexisting
// lookup contract while the ID spelling is made equivalent.
func TestFlowLookupParentReferenceIDFormsAndControls(t *testing.T) {
	org := storage.NewOrgState()
	parentID := storage.ID("a00000000000001")
	targetID := storage.ID("b00000000000001")
	parentDef := storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{"Ref__c": {APIName: "Ref__c", Type: storage.FieldReference, ReferenceTo: []string{"Target__c"}}, "Text__c": {APIName: "Text__c", Type: storage.FieldString}}}
	parent := storage.Record{ID: parentID, Object: "Parent__c", Fields: map[string]storage.Value{"Ref__c": storage.IDValue(targetID)}}
	org.Objects["Parent__c"] = storage.ObjectState{Definition: parentDef, Records: map[storage.ID]storage.Record{parentID: parent}}
	childDef := storage.ObjectDefinition{APIName: "Child__c", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, RelationshipName: "Parent__r", ReferenceTo: []string{"Parent__c"}}}}
	engine := NewEngine(&org)
	for _, text := range []string{string(parentID), storage.ID18(string(parentID))} {
		t.Run(fmt.Sprintf("id_length_%d", len(text)), func(t *testing.T) {
			source := storage.Record{Fields: map[string]storage.Value{"Parent__c": storage.IDValue(storage.ID(text))}}
			value, ok := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef)
			if !ok || value.Kind != storage.ValueID || value.ID != targetID {
				t.Fatalf("lookup failed for length %d: kind %v, supported %v", len(text), value.Kind, ok)
			}
		})
	}
	source := storage.Record{Fields: map[string]storage.Value{"Parent__c": storage.IDValue(parentID)}}
	parent.System.IsDeleted = true
	org.Objects["Parent__c"].Records[parentID] = parent
	for _, text := range []string{string(parentID), storage.ID18(string(parentID))} {
		source.Fields["Parent__c"] = storage.IDValue(storage.ID(text))
		value, ok := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef)
		_, mainFound := org.Objects["Parent__c"].Records[storage.ID(text)]
		if ok != mainFound || (ok && (value.Kind != storage.ValueID || value.ID != targetID)) {
			t.Fatalf("deleted parent differs from base for ID length %d", len(text))
		}
	}
	source.Fields["Parent__c"] = storage.IDValue(parentID)
	for _, path := range []string{"Parent__r.Text__c", "Parent__r.Ref__c.Name", "Parent__r.Unknown__c", "$Record.Parent__r.Ref__c"} {
		if _, ok := engine.flowLookupParentReferenceValue(path, source, childDef); ok {
			t.Fatalf("unsupported path became supported: %s", path)
		}
	}
	source.Fields["Parent__c"] = storage.NullValue()
	if value, ok := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef); !ok || value.Kind != storage.ValueNull {
		t.Fatal("null parent behavior changed")
	}
	source.Fields["Parent__c"] = storage.IDValue(storage.ID("a00000000000002"))
	if _, ok := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef); ok {
		t.Fatal("missing parent became supported")
	}

	// Main accepted exact keys even when two equivalent spellings coexist.
	longID := storage.ID(storage.ID18(string(parentID)))
	otherID := storage.ID("b00000000000002")
	longParent := parent
	longParent.Fields = map[string]storage.Value{"Ref__c": storage.IDValue(otherID)}
	org.Objects["Parent__c"].Records[longID] = longParent
	source.Fields["Parent__c"] = storage.IDValue(longID)
	if value, ok := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef); !ok || value.ID != otherID {
		t.Fatal("exact-key precedence changed")
	}
	delete(org.Objects["Parent__c"].Records, parentID)
	source.Fields["Parent__c"] = storage.IDValue(parentID)
	// Main used an exact map lookup for these uncaptured storage forms.
	// Compare the route result to that original lookup rather than broadening
	// the observed 18-character input / 15-character key behavior.
	for _, input := range []storage.ID{parentID, storage.ID(string(parentID) + "ZZZ")} {
		source.Fields["Parent__c"] = storage.IDValue(input)
		_, mainFound := org.Objects["Parent__c"].Records[input]
		_, found := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef)
		if found != mainFound {
			t.Fatalf("uncaptured input length %d: got found=%v, main=%v", len(input), found, mainFound)
		}
	}
}

// API 68 was not in the ID-form capture; retain the base exact-key lookup.
func TestFlowLookupParentReferenceUncapturedVersionMatchesMain(t *testing.T) {
	org := storage.NewOrgState()
	org.APIVersion = "68.0"
	parentID := storage.ID("a00000000000001")
	parentDef := storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{"Ref__c": {APIName: "Ref__c", Type: storage.FieldReference, ReferenceTo: []string{"Target__c"}}}}
	org.Objects["Parent__c"] = storage.ObjectState{Definition: parentDef, Records: map[storage.ID]storage.Record{parentID: {ID: parentID, Fields: map[string]storage.Value{"Ref__c": storage.IDValue(storage.ID("b00000000000001"))}}}}
	childDef := storage.ObjectDefinition{APIName: "Child__c", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, RelationshipName: "Parent__r", ReferenceTo: []string{"Parent__c"}}}}
	source := storage.Record{Fields: map[string]storage.Value{"Parent__c": storage.IDValue(storage.ID(storage.ID18(string(parentID))))}}
	engine := NewEngine(&org)
	if _, ok := engine.flowLookupParentReferenceValue("Parent__r.Ref__c", source, childDef); ok {
		t.Fatal("uncaptured API version differs from main exact-key lookup")
	}
}
