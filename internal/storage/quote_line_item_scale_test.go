package storage

import "testing"

// quote_totals native rows show QuoteLineItem Decimals at the describe scale.
func TestQuoteLineItemStubFieldsCarryDescribeScale(t *testing.T) {
	org := NewOrgState()
	ApplyProjectOrgShape(&org, []string{"Quotes"})
	definition := org.Objects["QuoteLineItem"].Definition
	for name, scale := range map[string]int{"Quantity": 2, "UnitPrice": 2, "Discount": 2, "Subtotal": 2, "TotalPrice": 2, "ListPrice": 2} {
		field := definition.Fields[name]
		if field.Scale != scale || !field.ScaleSpecified || field.Precision == 0 {
			t.Errorf("%s precision=%d scale=%d specified=%v", name, field.Precision, field.Scale, field.ScaleSpecified)
		}
	}
	if !StandardFieldDynamicPutReadOnly("Quote", "GrandTotal") || !StandardFieldDynamicPutReadOnly("QuoteLineItem", "Subtotal") {
		t.Error("derived quote totals must reject dynamic put")
	}
	if StandardFieldDynamicPutReadOnly("Quote", "Tax") || StandardFieldDynamicPutReadOnly("QuoteLineItem", "Quantity") {
		t.Error("writable quote fields must accept dynamic put")
	}
}
