package typesys

import (
	"strings"
	"testing"
)

func TestPlan7RichMessagingOverlayContainsNewPaymentContracts(t *testing.T) {
	byName := make(map[string]StandardSymbolSpec, len(plan7RichMessagingSymbolSpecs))
	for _, spec := range plan7RichMessagingSymbolSpecs {
		if !strings.HasPrefix(spec.Name, "RichMessaging.") {
			t.Fatalf("unrelated Plan 7 overlay type %q", spec.Name)
		}
		byName[strings.ToLower(spec.Name)] = spec
	}

	assertProperties := func(typeName string, names ...string) {
		t.Helper()
		spec, ok := byName[strings.ToLower(typeName)]
		if !ok {
			t.Fatalf("missing overlay type %s", typeName)
		}
		for _, name := range names {
			found := false
			for _, property := range spec.Properties {
				if strings.EqualFold(property.Name, name) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s missing property %s", typeName, name)
			}
		}
	}

	assertProperties("RichMessaging.CurrencyAmount", "currency")
	assertProperties("RichMessaging.OrderBeneficiary", "legalName", "taxIdentifierType", "taxIdentifierValue")
	assertProperties("RichMessaging.OrderContext", "orderBeneficiary", "paymentInitiation", "paymentMethodHints")
	assertProperties("RichMessaging.PaymentError", "code", "message")
	assertProperties("RichMessaging.PaymentTransaction", "amount", "refunds", "transactionId", "status")
	assertProperties("RichMessaging.ProcessPaymentStatusRequest", "amount", "error", "paymentTransaction")
	assertProperties("RichMessaging.ProcessPaymentStatusResult", "message", "status")
	assertProperties("RichMessaging.Refund", "amount", "reason", "refundId", "status")

	handler, ok := byName[strings.ToLower("RichMessaging.ProcessPaymentStatusHandler")]
	if !ok {
		t.Fatal("missing ProcessPaymentStatusHandler")
	}
	if len(handler.Methods) != 1 || !strings.EqualFold(handler.Methods[0].Name, "processPaymentStatus") {
		t.Fatalf("ProcessPaymentStatusHandler methods = %#v", handler.Methods)
	}

	foundMergedCurrency := false
	for _, symbol := range buildStandardPlatformSymbols() {
		if !strings.EqualFold(symbol.Namespace, "RichMessaging") || !strings.EqualFold(symbol.Name, "CurrencyAmount") {
			continue
		}
		for _, member := range symbol.Members {
			if strings.EqualFold(member.Name, "currency") {
				foundMergedCurrency = true
			}
		}
	}
	if !foundMergedCurrency {
		t.Fatal("RichMessaging.CurrencyAmount.currency was not merged into standard symbols")
	}
}
