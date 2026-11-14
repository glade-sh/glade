package storage

import (
	"encoding/json"
	"testing"
)

// Ordinary records remain byte-for-byte equivalent to the pre-sharing clone path.
func TestRuntimeCloneUnsharedNeighborMatchesFrozenDefinitionClone(t *testing.T) {
	org := NewOrgState()
	id := ID("001000000000001AAA")
	object := ObjectState{Definition: ObjectDefinition{APIName: "Account"}, Records: map[ID]Record{id: {ID: id, Object: "Account", Fields: map[string]Value{"Name": StringValue("neighbor")}}}}
	org.Objects["Account"] = object
	want, err := json.Marshal(object.CloneRuntimeFrozenDefinition())
	if err != nil {
		t.Fatal(err)
	}
	clone := org.CloneRuntimeFrozenShared()
	got, err := json.Marshal(clone.Objects["Account"])
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("clone = %s, prior clone = %s", got, want)
	}
}
