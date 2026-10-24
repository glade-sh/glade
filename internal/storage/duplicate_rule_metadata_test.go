package storage

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDuplicateRuleDefinitionCloneAndPersistenceRoutes(t *testing.T) {
	const source = `{"apiName":"Contact","duplicateRules":[{"name":"Grove_Email_Check","active":true,"actionOnInsert":"Allow","actionOnUpdate":"Allow","operationsOnInsert":["Report"],"matchRules":[{"object":"Contact","name":"Grove_Exact_Email","items":[{"field":"Email","method":"Exact","blankBehavior":"NullNotAllowed"}]}]}]}`
	var definition ObjectDefinition
	if err := json.Unmarshal([]byte(source), &definition); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"definition", "org", "runtime", "rollback", "persistence"} {
		t.Run(route, func(t *testing.T) {
			org := OrgState{Objects: map[string]ObjectState{"Contact": {Definition: definition}}}
			var clone ObjectDefinition
			switch route {
			case "definition":
				clone = definition.Clone()
			case "org":
				clone = org.Clone().Objects["Contact"].Definition
			case "runtime":
				clone = org.CloneRuntime().Objects["Contact"].Definition
			case "rollback":
				clone = org.CloneRollbackSnapshot().Objects["Contact"].Definition
			case "persistence":
				body, err := json.Marshal(org)
				if err != nil {
					t.Fatal(err)
				}
				var restored OrgState
				if err := json.Unmarshal(body, &restored); err != nil {
					t.Fatal(err)
				}
				clone = restored.Objects["Contact"].Definition
			}
			body, err := json.Marshal(clone)
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(source), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("duplicate metadata lost: %s", body)
			}
			rules := reflect.ValueOf(&clone).Elem().FieldByName("DuplicateRules")
			rules.Index(0).FieldByName("MatchRules").Index(0).FieldByName("Items").Index(0).FieldByName("Field").SetString("LastName")
			rules.Index(0).FieldByName("OperationsOnInsert").Index(0).SetString("Alert")
			original, _ := json.Marshal(definition)
			if string(original) != source {
				var originalValue any
				_ = json.Unmarshal(original, &originalValue)
				if !reflect.DeepEqual(originalValue, want) {
					t.Fatalf("clone mutation changed original: %s", original)
				}
			}
		})
	}
}
