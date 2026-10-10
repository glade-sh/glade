package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDMLRecalculatesSummaryFieldsOnParentReparent(t *testing.T) {
	org := testOrg()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Total__c": {APIName: "Total__c", Type: storage.FieldSummary, SummarizedField: "Child__c.Amount__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "sum"},
		},
	}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
		},
	}, Records: map[storage.ID]storage.Record{}}
	engine := NewEngine(&org)
	parents := engine.Insert([]storage.Record{{Object: "Parent__c"}, {Object: "Parent__c"}})
	if !parents[0].Success || !parents[1].Success {
		t.Fatalf("parent insert = %#v", parents)
	}
	child := engine.Insert([]storage.Record{{Object: "Child__c", Fields: map[string]storage.Value{
		"Parent__c": storage.IDValue(parents[0].ID), "Amount__c": storage.DecimalValue("5"),
	}}})
	if !child[0].Success {
		t.Fatalf("child insert = %#v", child)
	}
	updated := engine.Update([]storage.Record{{Object: "Child__c", ID: child[0].ID, Fields: map[string]storage.Value{
		"Parent__c": storage.IDValue(parents[1].ID),
	}}})
	if !updated[0].Success {
		t.Fatalf("child reparent = %#v", updated)
	}
	if got := org.Objects["Parent__c"].Records[parents[0].ID].Fields["Total__c"].Decimal; got != "0" {
		t.Fatalf("old parent total = %q, want 0", got)
	}
	if got := org.Objects["Parent__c"].Records[parents[1].ID].Fields["Total__c"].Decimal; got != "5" {
		t.Fatalf("new parent total = %q, want 5", got)
	}
}

func TestDMLRollupSummaryMatchesCommaSeparatedFilterValues(t *testing.T) {
	org := testOrg()
	org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Parent__c",
		Fields: map[string]storage.Field{
			"Total__c": {APIName: "Total__c", Type: storage.FieldSummary, SummarizedField: "Child__c.Amount__c", SummaryForeignKey: "Child__c.Parent__c", SummaryOperation: "sum", SummaryFilterItems: []storage.SummaryFilterItem{{Field: "Child__c.Status__c", Operation: "equals", Value: "Confirmed, Completed"}}},
		},
	}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Child__c",
		Fields: map[string]storage.Field{
			"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}},
			"Amount__c": {APIName: "Amount__c", Type: storage.FieldDecimal},
			"Status__c": {APIName: "Status__c", Type: storage.FieldString},
		},
	}, Records: map[storage.ID]storage.Record{}}
	engine := NewEngine(&org)
	parent := engine.Insert([]storage.Record{{Object: "Parent__c"}})
	if !parent[0].Success {
		t.Fatalf("parent insert = %#v", parent[0])
	}
	child := engine.Insert([]storage.Record{{Object: "Child__c", Fields: map[string]storage.Value{
		"Parent__c": storage.IDValue(parent[0].ID), "Amount__c": storage.DecimalValue("3"), "Status__c": storage.StringValue("Confirmed"),
	}}})
	if !child[0].Success {
		t.Fatalf("child insert = %#v", child[0])
	}
	if got := org.Objects["Parent__c"].Records[parent[0].ID].Fields["Total__c"].Decimal; got != "3" {
		t.Fatalf("filtered summary total = %q, want 3", got)
	}
}
