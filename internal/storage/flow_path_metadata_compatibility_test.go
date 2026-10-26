package storage

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlowPathMetadataImmediateOnlyJSONRoute(t *testing.T) {
	// Existing serialized rules keep their shape when no deferred paths exist.
	rule := FlowRule{Name: "Immediate", Steps: []FlowStep{{Kind: "reference", Target: "mark"}}}
	encoded, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["paths"]; present {
		t.Fatalf("immediate-only JSON acquired deferred metadata: %s", encoded)
	}
	var roundtrip FlowRule
	if err := json.Unmarshal(encoded, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rule, roundtrip) {
		t.Fatalf("immediate-only JSON changed: got=%+v want=%+v", roundtrip, rule)
	}
}
