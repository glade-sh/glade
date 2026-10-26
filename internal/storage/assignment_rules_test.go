package storage

import "testing"

func TestAssignmentRuleDefinitionCloneAndRecordOptionsRoutes(t *testing.T) {
	original := ObjectDefinition{AssignmentRules: []AssignmentRule{{Name: "Routing", Entries: []AssignmentRuleEntry{{AssignedTo: "First", Criteria: []WorkflowCriteriaItem{{Field: "Company", Value: "match"}}}}}}}
	copy := original.Clone()
	copy.AssignmentRules[0].Name = "Changed"
	copy.AssignmentRules[0].Entries[0].AssignedTo = "Second"
	copy.AssignmentRules[0].Entries[0].Criteria[0].Value = "miss"
	rule := original.AssignmentRules[0]
	if rule.Name != "Routing" || rule.Entries[0].AssignedTo != "First" || rule.Entries[0].Criteria[0].Value != "match" {
		t.Fatal("assignment metadata clone aliases its source")
	}
	record := Record{AssignmentRule: &AssignmentRuleSelection{UseDefault: true}}
	cloned := record.Clone()
	cloned.AssignmentRule.UseDefault = false
	if !record.AssignmentRule.UseDefault {
		t.Fatal("record options clone aliases its source")
	}
}

func TestAssignmentRuleStandardSchemaEnrichmentRoute(t *testing.T) {
	org := NewOrgState()
	org.Objects["Lead"] = ObjectState{Definition: ObjectDefinition{APIName: "Lead", AssignmentRules: []AssignmentRule{{Name: "Routing"}}}}
	EnsureStandardObject(&org, "Lead")
	rules := org.Objects["Lead"].Definition.AssignmentRules
	if len(rules) != 1 || rules[0].Name != "Routing" {
		t.Fatal("standard schema enrichment discarded assignment metadata")
	}
}
