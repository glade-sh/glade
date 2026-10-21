package apextest

import "testing"

func TestInlineSearchReturningOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "inline_search_returning.json", "InlineSearchReturning", 6)
}
