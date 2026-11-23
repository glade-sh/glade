package apextest

import "testing"

// NATIVE default currency direct insert and flow create: org.tsv lines 2-5,
// anonymous and isolated tests at API 62.0 and 67.0.
func TestPricebookEntryCurrencyRoutesNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "pricebook_entry_currency_routes", 4, nil)
}
