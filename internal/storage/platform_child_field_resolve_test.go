package storage

import "testing"

// Every platform child edge attached to an ordinary custom object must name a
// field that resolves on the child's runtime definition, so describing the
// ChildRelationship field token never fails.
func TestCustomPlatformChildEdgesResolveOnChildDefinitions(t *testing.T) {
	for _, parentName := range []string{"ChildProbe__c"} {
		checkPlatformChildEdges(t, ObjectDefinition{APIName: parentName})
	}
}

func checkPlatformChildEdges(t *testing.T, parent ObjectDefinition) {
	visited := 0
	VisitPlatformChildRelationships(parent, func(child string, relationship Relationship) {
		visited++
		if len(child) > 7 && child[len(child)-7:] == "__Share" {
			// The per-parent share entity is built with its parent by the loader.
			return
		}
		org := NewOrgState()
		EnsureStandardObject(&org, child)
		state, ok := org.Objects[child]
		if !ok {
			t.Errorf("%s edge %s: child %s not materialized", parent.APIName, relationship.ChildRelationship, child)
			return
		}
		if _, ok := ResolveFieldName(state.Definition, "", relationship.Field); !ok {
			t.Errorf("%s edge %s: field %s does not resolve on %s", parent.APIName, relationship.ChildRelationship, relationship.Field, child)
		}
	})
	if visited == 0 {
		t.Fatalf("%s: no platform child edges visited", parent.APIName)
	}
}

// A definition persisted by an older binary lacks the edge fields but carries
// the overlay marker; loading must repair it, and a second pass must be a no-op.
func TestPlatformChildEdgeFieldsRepairPersistedDefinition(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "FinanceTransaction")
	definition := org.Objects["FinanceTransaction"].Definition.Clone()
	delete(definition.Fields, "DestinationEntityId")
	if !StandardObjectFieldsNeedWrite(definition) {
		t.Fatal("definition missing an edge field is not reported for repair")
	}
	EnsureStandardObjectFields(&definition)
	field, ok := definition.Fields["DestinationEntityId"]
	if !ok || field.Type != FieldReference || len(field.ReferenceTo) != 9 || field.RelationshipName != "DestinationEntity" {
		t.Fatalf("repaired field = %+v", field)
	}
	if StandardObjectFieldsNeedWrite(definition) {
		t.Fatal("repair is not idempotent")
	}
}
