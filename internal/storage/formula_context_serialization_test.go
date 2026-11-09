package storage

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"
)

func TestFormulaQueryContextIsNotSerialized(t *testing.T) {
	original := NewOrgState()
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	org := original.WithFormulaContext("005000000000001", map[string]Value{"Permissions": StringValue("Bypass")})
	after, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("query-only context altered persisted JSON")
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(org); err != nil {
		t.Fatal(err)
	}
	var decoded OrgState
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if user, fields := decoded.FormulaContext(); user != "" || fields != nil {
		t.Fatal("query-only context survived gob serialization")
	}
	if user, fields := original.FormulaContext(); user != "" || fields != nil {
		t.Fatal("query context mutated the source org")
	}
}
