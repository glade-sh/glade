package apextest

import "testing"

// NATIVE custom price without a standard price: org.tsv lines 2-5,
// anonymous and isolated test, API 62.0 and 67.0.
func TestPricebookEntryStandardPriceNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "pricebook_entry_standard_price", 4, nil)
}
