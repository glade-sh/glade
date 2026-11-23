package apextest

import "testing"

// NATIVE standard price and matching-product control: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestPricebookEntryStandardPriceControlsNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "pricebook_entry_standard_price_controls", 2, nil)
}
