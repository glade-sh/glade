package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// The public Opportunity object reference lists Create and Update for quantity:
// https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_opportunity.htm
func TestOpportunityQuantityCreateAndUpdate(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	engine := NewEngine(&org)
	fields := opportunityQuantityFields()
	fields["TotalOpportunityQuantity"] = storage.DecimalValue("3")
	inserted := engine.Insert([]storage.Record{{Object: "Opportunity", Fields: fields}})
	if !inserted[0].Success {
		t.Fatalf("quantity insert: %+v", inserted[0])
	}
	if got := org.Objects["Opportunity"].Records[inserted[0].ID].Fields["TotalOpportunityQuantity"]; got.Decimal != "3" {
		t.Fatalf("inserted quantity: %+v", got)
	}
	changed := engine.Update([]storage.Record{{ID: inserted[0].ID, Object: "Opportunity", Fields: map[string]storage.Value{"TotalOpportunityQuantity": storage.DecimalValue("8")}}})
	if !changed[0].Success {
		t.Fatalf("quantity update: %+v", changed[0])
	}
	if got := org.Objects["Opportunity"].Records[inserted[0].ID].Fields["TotalOpportunityQuantity"]; got.Decimal != "8" {
		t.Fatalf("updated quantity: %+v", got)
	}
	fields = opportunityQuantityFields()
	fields["ExpectedRevenue"] = storage.DecimalValue("5")
	rejected := engine.Insert([]storage.Record{{Object: "Opportunity", Fields: fields}})
	assertDMLErrorDetail(t, rejected[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "ExpectedRevenue")
}

func TestOpportunityQuantityPreservesExplicitWriteRestrictions(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Opportunity")
	state := org.Objects["Opportunity"]
	field := state.Definition.Fields["TotalOpportunityQuantity"]
	field.Createable = storage.BoolFlag(false)
	field.Updateable = storage.BoolFlag(false)
	state.Definition.Fields["TotalOpportunityQuantity"] = field
	storage.EnsureStandardObjectFields(&state.Definition)
	org.Objects["Opportunity"] = state
	fields := opportunityQuantityFields()
	fields["TotalOpportunityQuantity"] = storage.DecimalValue("3")
	engine := NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "Opportunity", Fields: fields}})
	assertDMLErrorDetail(t, inserted[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "TotalOpportunityQuantity")
	seeded := engine.Insert([]storage.Record{{Object: "Opportunity", Fields: opportunityQuantityFields()}})
	if !seeded[0].Success {
		t.Fatalf("read-only control seed: %+v", seeded[0])
	}
	updated := engine.Update([]storage.Record{{ID: seeded[0].ID, Object: "Opportunity", Fields: map[string]storage.Value{"TotalOpportunityQuantity": storage.DecimalValue("8")}}})
	assertDMLErrorDetail(t, updated[0], "INVALID_FIELD_FOR_INSERT_UPDATE", "TotalOpportunityQuantity")
}

func opportunityQuantityFields() map[string]storage.Value {
	return map[string]storage.Value{"Name": storage.StringValue("sample"), "StageName": storage.StringValue("Prospecting"), "CloseDate": storage.DateValue("2026-10-15")}
}
