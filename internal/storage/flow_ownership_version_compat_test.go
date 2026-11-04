package storage

import (
	"encoding/json"
	"testing"
)

func TestProjectFlowOwnershipVersionJSONCompatibility(t *testing.T) {
	var legacy MetadataRegistry
	if err := json.Unmarshal([]byte(`{"flows":[{"name":"Old"}]}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.ProjectFlowOwnershipVersion != 0 {
		t.Fatal("legacy snapshot has an unexpected version")
	}
	legacy.ProjectFlowOwnershipVersion = 1
	legacy.ProjectFlowRootID = "12:34"
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var current MetadataRegistry
	if err := json.Unmarshal(encoded, &current); err != nil {
		t.Fatal(err)
	}
	if current.ProjectFlowOwnershipVersion != 1 || current.ProjectFlowRootID != "12:34" || len(current.Flows) != 1 {
		t.Fatalf("round trip=%+v", current)
	}
}
