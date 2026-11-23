package apextest

import "testing"

// NATIVE default currency custom price: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestPricebookEntryCurrencyNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "pricebook_entry_currency", 2, nil)
}
