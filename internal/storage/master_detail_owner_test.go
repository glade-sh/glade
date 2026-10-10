package storage

import (
	"reflect"
	"testing"
)

func TestMasterDetailCustomOwnerOverlay(t *testing.T) {
	for _, sharing := range []string{"ReadWrite", "ControlledByParent"} {
		t.Run(sharing, func(t *testing.T) {
			def := ObjectDefinition{APIName: "OwnedDetail__c", SharingModel: sharing}
			EnsureStandardObjectFields(&def)
			_, present := ResolveFieldName(def, "", "OwnerId")
			if present != (sharing == "ReadWrite") {
				t.Fatalf("OwnerId present=%v", present)
			}
			if sharing == "ControlledByParent" {
				def.Fields["OwnerId"] = Field{APIName: "OwnerId", Type: FieldReference}
				def.Relations = append(def.Relations, Relationship{Field: "OwnerId", ParentRelationship: "Owner"})
				if !StandardObjectFieldsNeedWrite(def) {
					t.Fatal("stale overlay owner not repaired")
				}
				EnsureStandardObjectFields(&def)
				if _, present := ResolveFieldName(def, "", "OwnerId"); present {
					t.Fatal("stale OwnerId retained")
				}
				for _, relation := range def.Relations {
					if relation.Field == "OwnerId" {
						t.Fatal("stale Owner relationship retained")
					}
				}
			}
		})
	}
}

func TestMasterDetailOwnerRepairPreservesSharedSiblingDefinition(t *testing.T) {
	original := NewOrgState()
	name := "OwnedDetail__c"
	original.Objects[name] = ObjectState{
		Definition: ObjectDefinition{APIName: name, SharingModel: "ControlledByParent"},
		Records:    make(map[ID]Record),
	}
	EnsureStandardObject(&original, name)
	state := original.Objects[name]
	// Emulate a complete older overlaid definition, shared by runtime clones.
	state.Definition.Fields["OwnerId"] = Field{APIName: "OwnerId", Type: FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"}
	state.Definition.Relations = append([]Relationship{{Field: "OwnerId", ParentRelationship: "Owner"}}, state.Definition.Relations...)
	original.Objects[name] = state
	before := state.Definition.Clone()
	sibling := original.CloneRuntimeFrozenShared()
	EnsureStandardObject(&sibling, name)
	repaired := sibling.Objects[name].Definition
	if _, present := ResolveFieldName(repaired, "", "OwnerId"); present {
		t.Fatal("sibling retained stale OwnerId")
	}
	for _, relation := range repaired.Relations {
		if relation.Field == "OwnerId" {
			t.Fatal("sibling retained stale owner relationship")
		}
	}
	if len(repaired.Relations) != len(before.Relations)-1 {
		t.Fatal("repair dropped unrelated relationships")
	}
	if !reflect.DeepEqual(original.Objects[name].Definition, before) {
		t.Fatal("repair mutated shared original fields or relationship backing array")
	}
}
