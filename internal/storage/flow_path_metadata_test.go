package storage

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFlowPathMetadataJSONAndCloneRoutes(t *testing.T) {
	// The public JSON boundary keeps this regression compilable before the
	// model has a Paths field. Clone isolation is a local storage contract.
	const source = `{"name":"DeferredParcel","paths":[{"name":"DelayedPath","kind":"async","steps":[{"kind":"reference","target":"mark"}],"stepTargets":{"mark":[{"kind":"fieldUpdate","fieldUpdates":[{"field":"Marker__c","literalValue":"true"}]}]}}]}`
	var rule FlowRule
	if err := json.Unmarshal([]byte(source), &rule); err != nil {
		t.Fatal(err)
	}
	assertPaths := func(t *testing.T, candidate FlowRule) {
		t.Helper()
		encoded, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		type projection struct {
			Paths []struct {
				Name, Kind  string
				Steps       []FlowStep
				StepTargets map[string][]FlowStep
			}
		}
		var want, got projection
		if err := json.Unmarshal([]byte(source), &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Paths, want.Paths) {
			t.Fatalf("deferred path JSON lost: got=%s; want=%s", encoded, source)
		}
	}
	t.Run("json_roundtrip", func(t *testing.T) { assertPaths(t, rule) })
	for _, route := range []string{"definition", "org", "runtime", "frozen_definition_metadata"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			org.Metadata.Flows = []FlowRule{rule}
			org.Metadata.FlowDeclarations = []FlowRule{rule}
			org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", FlowRules: []FlowRule{rule}}}
			var candidates []FlowRule
			switch route {
			case "definition":
				candidates = org.Objects["Account"].Definition.Clone().FlowRules
			default:
				var cloned OrgState
				switch route {
				case "org":
					cloned = org.Clone()
				case "runtime":
					cloned = org.CloneRuntime()
				case "frozen_definition_metadata":
					cloned = org.CloneRuntimeFrozenDefinition()
				}
				candidates = append(candidates, cloned.Metadata.Flows...)
				candidates = append(candidates, cloned.Metadata.FlowDeclarations...)
				if route != "frozen_definition_metadata" {
					candidates = append(candidates, cloned.Objects["Account"].Definition.FlowRules...)
				}
			}
			for i := range candidates {
				assertPaths(t, candidates[i])
				paths := reflect.ValueOf(&candidates[i]).Elem().FieldByName("Paths")
				paths.Index(0).FieldByName("Name").SetString("Changed")
				steps := paths.Index(0).FieldByName("Steps").Interface().([]FlowStep)
				steps[0].Target = "changed"
				targets := paths.Index(0).FieldByName("StepTargets").Interface().(map[string][]FlowStep)
				targets["mark"][0].FieldUpdates[0].LiteralValue = "false"
				delete(targets, "mark")
				assertPaths(t, rule)
				assertPaths(t, org.Metadata.Flows[0])
				assertPaths(t, org.Metadata.FlowDeclarations[0])
				assertPaths(t, org.Objects["Account"].Definition.FlowRules[0])
			}
		})
	}
}
