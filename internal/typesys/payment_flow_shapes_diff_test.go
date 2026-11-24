package typesys

import (
	"reflect"
	"strings"
	"testing"
)

// Rebuild the main symbol table without the four new overlays, and compare
// every other symbol so unobserved platform shapes keep their previous surface.
func TestPaymentFlowShapesUncapturedSymbolsUnchanged(t *testing.T) {
	changed := map[string]string{
		"commercepayments.salerequest":                   "commercepayments.PaymentGatewayRequest",
		"commercepayments.authorizationreversalrequest":  "commercepayments.PaymentGatewayRequest",
		"commercepayments.saleresponse":                  "commercepayments.GatewayResponse",
		"commercepayments.authorizationreversalresponse": "commercepayments.GatewayResponse",
	}
	specs := standardPlatformSymbolSpecList(false)
	baselineSpecs := make([]StandardSymbolSpec, 0, len(specs))
	removed := 0
	for _, spec := range specs {
		want, found := changed[strings.ToLower(spec.Name)]
		if found && len(spec.Interfaces) == 1 && spec.Interfaces[0] == want {
			removed++
			continue
		}
		baselineSpecs = append(baselineSpecs, spec)
	}
	if removed != len(changed) {
		t.Fatalf("expected four added overlays, found %d", removed)
	}
	baseline := standardSymbolsFromSpecs(baselineSpecs, newStandardSymbolLists())
	actual := standardSymbolsFromSpecs(specs, newStandardSymbolLists())
	filter := func(symbols []TypeSymbol) map[string]TypeSymbol {
		out := make(map[string]TypeSymbol, len(symbols))
		for _, symbol := range symbols {
			name := strings.ToLower(symbol.Namespace + "." + symbol.Name)
			if _, ok := changed[name]; !ok {
				out[name] = symbol
			}
		}
		return out
	}
	before, after := filter(baseline), filter(actual)
	if len(before) != len(after) {
		t.Fatalf("uncaptured symbol count: before %d, after %d", len(before), len(after))
	}
	for name, want := range before {
		got, ok := after[name]
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("uncaptured symbol %s changed", name)
		}
	}
}
