package sema

import (
	"strings"
	"testing"
)

func TestCartCalculateExecutorMockNativeOverridePoints(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"OverrideProbe.cls": `
global class OverrideProbe extends CartExtension.CartCalculateExecutorMock {
  global override void prices(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void promotions(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void inventory(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void shipping(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void shippingPromotions(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void tax(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void postShipping(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void defaultInventory(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void defaultPrices(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void defaultPromotions(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void defaultShipping(CartExtension.CartCalculateCalculatorRequest request) {}
  global override void defaultTax(CartExtension.CartCalculateCalculatorRequest request) {}
}
`,
	}, "65.0")
	if result.HasErrors() {
		t.Fatalf("native CartCalculateExecutorMock override points were rejected: %#v", result.Diagnostics)
	}
}

func TestCartCalculateExecutorMockOverrideCorrectionKeepsNonVirtualControl(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"ControlBase.cls":  `public virtual class ControlBase { public void run() {} }`,
		"ControlChild.cls": `public class ControlChild extends ControlBase { public override void run() {} }`,
	}, "65.0")
	var got []string
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA016" {
			got = append(got, diag.Message)
		}
	}
	if len(got) != 1 || !strings.Contains(got[0], `"run"`) {
		t.Fatalf("non-virtual override control diagnostics = %#v, want only the run override diagnostic", got)
	}
}
