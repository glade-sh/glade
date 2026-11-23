package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Uncaptured direct inactive custom prices keep the pre-change insert result.
func TestPricebookInactiveInsertUncapturedNeighbor(t *testing.T) {
	org := testOrg()
	for _, name := range []string{"Product2", "Pricebook2", "PricebookEntry"} {
		storage.EnsureStandardObject(&org, name)
	}
	engine := NewEngine(&org)
	product := scopeInsert(t, &engine, "Product2", map[string]storage.Value{"Name": storage.StringValue("Synthetic"), "IsActive": storage.BooleanValue(true)})
	book := scopeInsert(t, &engine, "Pricebook2", map[string]storage.Value{"Name": storage.StringValue("Custom"), "IsActive": storage.BooleanValue(true)})
	result := engine.Insert([]storage.Record{{Object: "PricebookEntry", Fields: map[string]storage.Value{"Product2Id": storage.IDValue(product), "Pricebook2Id": storage.IDValue(book), "UnitPrice": storage.DecimalValue("5"), "IsActive": storage.BooleanValue(false)}}})
	if len(result) != 1 || !result[0].Success {
		t.Fatalf("inactive insert changed: %+v", result)
	}
}

// A different endpoint version remains unsupported, even inside a template.
func TestFlowAPIUncapturedVersionNeighbor(t *testing.T) {
	org := testOrg()
	engine := NewEngine(&org)
	engine.FlowFaultInTest = func() bool { return true }
	definition := org.Objects["Account"].Definition
	definition.FlowRules = []storage.FlowRule{{Name: "Probe", TriggerType: "RecordAfterSave", RecordTriggerType: "Create"}}
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{}}
	frame := engine.newFlowStepFrame("Probe", &record, definition, nil, map[string]string{"body": "{!$Api.Partner_Server_URL_550}"}, nil)
	if got, ok := engine.flowFrameReferenceValue("body", frame); ok {
		t.Fatalf("uncaptured template resolved: %+v", got)
	}
}
