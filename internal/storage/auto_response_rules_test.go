package storage

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAutoResponseDefinitionCloneAndStandardEnrichment(t *testing.T) {
	original := ObjectDefinition{AutoResponseRules: []AutoResponseRule{{Name: "Response", Entries: []AutoResponseRuleEntry{{Subject: "Synthetic", Criteria: []WorkflowCriteriaItem{{Field: "Company", Value: "match"}}}}}}}
	cloned := original.Clone()
	cloned.AutoResponseRules[0].Entries[0].Subject = "changed"
	cloned.AutoResponseRules[0].Entries[0].Criteria[0].Value = "miss"
	if original.AutoResponseRules[0].Entries[0].Subject != "Synthetic" || original.AutoResponseRules[0].Entries[0].Criteria[0].Value != "match" {
		t.Fatal("auto-response metadata clone aliases its source")
	}
	org := NewOrgState()
	org.Objects["Lead"] = ObjectState{Definition: original}
	EnsureStandardObject(&org, "Lead")
	if len(org.Objects["Lead"].Definition.AutoResponseRules) != 1 {
		t.Fatal("standard enrichment discarded auto-response metadata")
	}
}

func TestAutoResponseDefinitionOldAndNewJSONCompatibility(t *testing.T) {
	var old ObjectDefinition
	if err := json.Unmarshal([]byte(`{"apiName":"Lead","label":"Lead"}`), &old); err != nil {
		t.Fatal(err)
	}
	if len(old.AutoResponseRules) != 0 {
		t.Fatal("old definition gained response rules")
	}
	oldBytes, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oldBytes), "autoResponseRules") {
		t.Fatal("empty response rules changed old JSON shape")
	}
	old.AutoResponseRules = []AutoResponseRule{{Object: "Lead", Name: "Synthetic", Active: true, Entries: []AutoResponseRuleEntry{{Subject: "Response", Criteria: []WorkflowCriteriaItem{{Field: "Company", Operation: "equals", Value: "Synthetic"}}}}}}
	encoded, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	var restored ObjectDefinition
	if err = json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.AutoResponseRules) != 1 || restored.AutoResponseRules[0].Entries[0].Criteria[0].Value != "Synthetic" {
		t.Fatal("response metadata did not round trip")
	}
}
