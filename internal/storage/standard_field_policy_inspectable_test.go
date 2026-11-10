package storage

import (
	"encoding/json"
	"os"
	"testing"
)

// The shipped sidecar must be readable by range-based secret scanners and
// must be the same policy consumed by the runtime.
func TestStandardFieldPolicyInspectableJSON(t *testing.T) {
	data, err := os.ReadFile("standard_field_policy.json")
	if err != nil {
		t.Fatalf("read inspectable policy: %v", err)
	}
	if !json.Valid(data) || !json.Valid(standardFieldPolicyData) {
		t.Fatal("policy file and embedded runtime policy must be valid JSON")
	}
	if string(data) != string(standardFieldPolicyData) {
		t.Fatal("runtime policy differs from inspectable policy")
	}
}
