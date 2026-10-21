package storage

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

func TestFlowStepTargetsCloneIsolation(t *testing.T) {
	rule := FlowRule{
		Name:  "SharedPath",
		Steps: []FlowStep{{Kind: "reference", Target: "join"}},
		StepTargets: map[string][]FlowStep{
			"join": {{Kind: "decision", Branches: []FlowBranch{{
				Name:     "Continue",
				Criteria: []WorkflowCriteriaItem{{Field: "Enabled__c", Value: "true"}},
				Steps: []FlowStep{{Kind: "action", Action: FlowAction{
					Name: "RecordAction", Inputs: []WorkflowFieldUpdate{{Field: "Value__c", LiteralValue: "original"}},
				}}},
			}}}},
		},
	}
	org := NewOrgState()
	org.Metadata.Flows = []FlowRule{rule}
	definition := ObjectDefinition{APIName: "SharedEntry__c", FlowRules: []FlowRule{rule}}
	for name, clone := range map[string]FlowRule{
		"definition":       definition.Clone().FlowRules[0],
		"org":              org.Clone().Metadata.Flows[0],
		"runtime":          org.CloneRuntime().Metadata.Flows[0],
		"frozenDefinition": org.CloneRuntimeFrozenDefinition().Metadata.Flows[0],
		"rollback":         org.CloneRollbackSnapshot().Metadata.Flows[0],
	} {
		t.Run(name, func(t *testing.T) {
			if !reflect.DeepEqual(clone, rule) {
				t.Fatal("clone changed the compact Flow model")
			}
			clone.Steps[0].Target = "different"
			clone.StepTargets["extra"] = []FlowStep{{Kind: "reference", Target: "join"}}
			join := clone.StepTargets["join"]
			join[0].Branches[0].Criteria[0].Value = "false"
			join[0].Branches[0].Steps[0].Action.Inputs[0].LiteralValue = "changed"
			if rule.Steps[0].Target != "join" || len(rule.StepTargets) != 1 {
				t.Fatal("clone shares the root steps or target map")
			}
			branch := rule.StepTargets["join"][0].Branches[0]
			if branch.Criteria[0].Value != "true" || branch.Steps[0].Action.Inputs[0].LiteralValue != "original" {
				t.Fatal("clone shares a target's nested payload")
			}
		})
	}
}

func TestFlowStepTargetsJSONAndGobRoundTrip(t *testing.T) {
	rule := FlowRule{Name: "SharedPath", Steps: []FlowStep{{Kind: "reference", Target: "node0", VisitKey: "node0"}}, StepTargets: map[string][]FlowStep{}}
	for i := 0; i < 10; i++ {
		next := fmt.Sprintf("node%d", i+1)
		rule.StepTargets[fmt.Sprintf("node%d", i)] = []FlowStep{{Kind: "decision", Branches: []FlowBranch{
			{Name: "Selected", Formula: "true", Steps: []FlowStep{{Kind: "reference", Target: next, VisitKey: next}}},
			{Name: "Default", Default: true, Steps: []FlowStep{{Kind: "reference", Target: next, VisitKey: next}}},
		}}}
	}
	rule.StepTargets["node10"] = []FlowStep{{Kind: "assignment", Assignment: FlowAssignment{Target: "Result", LiteralValue: "complete"}}, {Kind: "reference", Target: "node0", VisitKey: "node0"}}
	encoded, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 100_000 {
		t.Fatalf("shared target model expanded to %d JSON bytes", len(encoded))
	}
	cloned, err := json.Marshal(ObjectDefinition{FlowRules: []FlowRule{rule}}.Clone().FlowRules[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, cloned) {
		t.Fatal("cloning changed the serialized shared-target graph")
	}
	var fromJSON FlowRule
	if err := json.Unmarshal(encoded, &fromJSON); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rule, fromJSON) {
		t.Fatal("JSON round trip changed target references or payloads")
	}
	var binary bytes.Buffer
	if err := gob.NewEncoder(&binary).Encode(rule); err != nil {
		t.Fatal(err)
	}
	if binary.Len() > 100_000 {
		t.Fatalf("shared target model expanded to %d gob bytes", binary.Len())
	}
	var fromGob FlowRule
	if err := gob.NewDecoder(&binary).Decode(&fromGob); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rule, fromGob) {
		t.Fatal("gob round trip changed target references or payloads")
	}
}

func TestFlowLegacyJSONShape(t *testing.T) {
	legacy := FlowRule{Name: "LegacyPath", Steps: []FlowStep{{Kind: "assignment", Assignment: FlowAssignment{Target: "Result", LiteralValue: "complete"}}}}
	encoded, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["stepTargets"]; present {
		t.Fatal("legacy Flow gained a stepTargets field")
	}
	var steps []map[string]json.RawMessage
	if err := json.Unmarshal(fields["steps"], &steps); err != nil {
		t.Fatal(err)
	}
	if _, present := steps[0]["target"]; present {
		t.Fatal("legacy step gained a reference target field")
	}
	var decoded FlowRule
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy, decoded) {
		t.Fatal("legacy nested Flow changed after JSON round trip")
	}
}
