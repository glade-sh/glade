package typesys

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The digest was captured from the requested main base before the payment-flow
// overlay. It covers every standard symbol except the four native-probed types.
func TestPaymentFlowShapesUncapturedSymbolsUnchanged(t *testing.T) {
	var keep []TypeSymbol
	for _, symbol := range StandardPlatformSymbols() {
		switch strings.ToLower(symbol.Name) {
		case "commercepayments.salerequest", "commercepayments.saleresponse", "commercepayments.authorizationreversalrequest", "commercepayments.authorizationreversalresponse":
			continue
		}
		keep = append(keep, symbol)
	}
	data, err := json.Marshal(keep)
	if err != nil {
		t.Fatal(err)
	}
	got := fmt.Sprintf("%x", sha256.Sum256(data))
	const baseline = "1d3af9ac0e17a9eb01ee686bcb4f025f97883f9a02533dcbe68193d0110be642"
	if got != baseline {
		t.Fatalf("uncaptured standard symbols differ from main: %s", got)
	}
}
