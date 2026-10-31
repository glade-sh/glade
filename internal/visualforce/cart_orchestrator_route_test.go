package visualforce

import "testing"

// Salesforce accepts the subclass call at API 62 and 67:
// internal/sema/testdata/cart_orchestrator_methods/org.tsv:2,7.
func TestCartOrchestratorControllerCompileRoute(t *testing.T) {
	p := writeControllerPageProject(t, `public class ProbeController extends CartExtension.CartCalculate {
  public String value { get; set; }
  public override void calculate(CartExtension.CartCalculateOrchestratorRequest r) {
    shippingPromotions((CartExtension.CartCalculateCalculatorRequest)null);
  }
}`)
	compilation := gateLikeCompilation(t, p)
	if got := diagnosticMessages(compilation); len(got) != 0 {
		t.Fatalf("controller diagnostics: %v", got)
	}
	for i := 0; i < 2; i++ {
		if got := PageNamesWithControllerCompilation(p, compilation); len(got) != 2 {
			t.Fatalf("page registration pass %d: %v", i, got)
		}
	}
}
