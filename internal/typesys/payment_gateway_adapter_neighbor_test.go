package typesys

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// The uncaptured adjacent interface candidate retains its baseline platform shape.
func TestPaymentGatewayAdapterNeighborUnchanged(t *testing.T) {
	symbol := requireStandardSymbol(t, StandardPlatformSymbols(), "commercepayments.PaymentGatewayAsyncAdapter")
	if symbol.Kind != apexast.DeclarationClass {
		t.Fatalf("neighbor kind = %s, want baseline class", symbol.Kind)
	}
	data, err := json.Marshal(struct {
		Symbol                    TypeSymbol
		ConstructorsAuthoritative bool
	}{symbol, symbol.ConstructorsAuthoritative})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("neighbor=%s", data)
}
