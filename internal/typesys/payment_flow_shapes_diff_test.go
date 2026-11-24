package typesys

import (
	"strings"
	"testing"
)

// The captured constructor and assignment cases do not alter the shared
// platform hierarchy used for all other callers.
func TestPaymentFlowShapesUncapturedSymbolsUnchanged(t *testing.T) {
	expected := map[string]bool{
		"commercepayments.salerequest":                   true,
		"commercepayments.authorizationreversalrequest":  true,
		"commercepayments.saleresponse":                  true,
		"commercepayments.authorizationreversalresponse": true,
	}
	symbols := standardSymbolsFromSpecs(standardPlatformSymbolSpecList(false), newStandardSymbolLists())
	seen := 0
	for _, symbol := range symbols {
		key := strings.ToLower(symbol.Namespace + "." + symbol.Name)
		if !expected[key] {
			continue
		}
		seen++
		if len(symbol.Interfaces) != 0 {
			t.Errorf("base hierarchy for %s changed: %v", key, symbol.Interfaces)
		}
	}
	if seen != len(expected) {
		t.Fatalf("found %d payment symbols, want %d", seen, len(expected))
	}
}
