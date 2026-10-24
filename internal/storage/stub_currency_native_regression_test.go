package storage

import "testing"

func TestStandardStubCurrencyFreshAndMarkedDefinitionRoutes(t *testing.T) {
	for _, route := range []string{"fresh", "marked", "org"} {
		t.Run(route, func(t *testing.T) {
			definition, ok := StandardObjectDefinition("OpportunityLineItem")
			if !ok {
				t.Fatal("missing standard line-item definition")
			}
			if route != "fresh" {
				definition = definition.Clone()
				field := definition.Fields["TotalPrice"]
				field.DisplayType = "DOUBLE"
				definition.Fields["TotalPrice"] = field
				if !StandardObjectFieldsNeedWrite(definition) {
					t.Error("marked definition with stub currency type needs repair")
				}
				if got, want := standardFieldsOverlayNeedsRefresh(definition, ""), legacyStandardFieldsOverlayNeedsRefresh(definition, ""); got != want {
					t.Errorf("cached repair check=%v; uncached=%v", got, want)
				}
				if route == "org" {
					org := NewOrgState()
					org.Objects["OpportunityLineItem"] = ObjectState{Definition: definition}
					EnsureStandardObject(&org, "OpportunityLineItem")
					definition = org.Objects["OpportunityLineItem"].Definition
				} else {
					EnsureStandardObjectFields(&definition)
				}
			}
			for name, display := range map[string]string{"TotalPrice": "CURRENCY", "UnitPrice": "CURRENCY", "ListPrice": "CURRENCY", "Quantity": "DOUBLE"} {
				field := definition.Fields[name]
				if field.DisplayType != display {
					t.Errorf("native %s=%s; got %s", name, display, field.DisplayType)
				}
			}
			if StandardObjectFieldsNeedWrite(definition) {
				t.Error("repaired currency overlay still requests a write")
			}
		})
	}
}

func TestStandardStubCurrencyRepairPreservesExplicitFieldRoute(t *testing.T) {
	definition := ObjectDefinition{APIName: "OpportunityLineItem", Fields: map[string]Field{
		"TotalPrice":       {APIName: "TotalPrice", Type: FieldDecimal, DisplayType: "PERCENT", Updateable: BoolFlag(false)},
		"NativeControl__c": {APIName: "NativeControl__c", Type: FieldDecimal, DisplayType: "DOUBLE"},
	}}
	EnsureStandardObjectFields(&definition)
	field := definition.Fields["TotalPrice"]
	if field.DisplayType != "PERCENT" || field.Updateable == nil || *field.Updateable {
		t.Fatalf("explicit field metadata changed: %+v", field)
	}
	if field := definition.Fields["NativeControl__c"]; field.DisplayType != "DOUBLE" {
		t.Fatalf("uncaptured field changed: %+v", field)
	}
}
