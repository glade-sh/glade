package vm

import (
	"encoding/json"
	"testing"
)

func TestCase339JSONBudgetSnapshotReflectsKeySet(t *testing.T) {
	program, err := CompileAnonymous(`Map<String, Integer> m = new Map<String, Integer>{'a' => 1}; Set<String> keys = m.keySet(); m.put('b', 2);`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := New(nil).Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	value := result.Vars["keys"]
	snapshot := value.JSONBudgetSnapshot()
	if len(snapshot.Set) != 2 {
		t.Fatalf("snapshot has %d keys, want 2", len(snapshot.Set))
	}
	originalJSON, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if string(originalJSON) != string(snapshotJSON) {
		t.Fatalf("snapshot JSON differs: %s != %s", originalJSON, snapshotJSON)
	}
}
