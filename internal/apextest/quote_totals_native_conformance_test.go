package apextest

import "testing"

func TestQuoteTotalsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "quote_totals", "QuoteTotalsProbeTest", "62.0", "67.0")
}

func TestQuoteTotalsFollowupNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "quote_totals_followup", "QuoteFollowProbeTest", "62.0", "67.0")
}
