package storage

import (
	"encoding/json"
	"testing"
)

func TestFlowMergeConstantsCloneAndJSON(t *testing.T) {
	msg := FlowCustomErrorMessage{Message: "{!large}", NumberConstants: map[string]string{"large": "1234.50"}}
	definition := ObjectDefinition{FlowRules: []FlowRule{{Steps: []FlowStep{{CustomError: FlowCustomError{Messages: []FlowCustomErrorMessage{msg}}}}}}}
	copy := definition.Clone()
	copy.FlowRules[0].Steps[0].CustomError.Messages[0].NumberConstants["large"] = "changed"
	if got := definition.FlowRules[0].Steps[0].CustomError.Messages[0].NumberConstants["large"]; got != "1234.50" {
		t.Fatalf("clone changed original: %q", got)
	}
	b, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ObjectDefinition
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if got := decoded.FlowRules[0].Steps[0].CustomError.Messages[0].NumberConstants["large"]; got != "1234.50" {
		t.Fatalf("roundtrip: %q", got)
	}
}
