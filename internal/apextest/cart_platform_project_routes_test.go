package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

// Synthetic calculator/helper shape backed by the API62/67 accepted deployment
// rows in ../sema/testdata/cart_extension_platform_types/org.tsv:10-15.
func TestCartPlatformCalculatorProjectRoutesNativeConformance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{
				"OrchardCartHelper.cls": `public class OrchardCartHelper {
 public static CartExtension.CommerceObject itemAsCommerce(CartExtension.CartItem item) { return item; }
 public static CartExtension.CommerceObject groupAsCommerce(CartExtension.CartDeliveryGroup groupValue) { return groupValue; }
 public static void updateItems(CartExtension.CartItemList items, CartExtension.CartDeliveryGroup groupValue, CartExtension.CartItem item) {
  items.add(groupValue); items.remove(groupValue); items.add(item); items.remove(item);
 }
}`,
				"OrchardCartCalculator.cls": `public class OrchardCartCalculator extends CartExtension.PricingCartCalculator {
 public static Integer calls=0;
 public override void calculate(CartExtension.CartCalculateCalculatorRequest request) {
  calls++;
  if(request==null) return;
  CartExtension.Cart cart=request.getCart();
  OrchardCartHelper.updateItems(cart.getCartItems(),new CartExtension.CartDeliveryGroup(),null);
 }
}`,
				"OrchardCartProof.cls": `@IsTest private class OrchardCartProof {
 @IsTest static void inheritedCalculatorHelperRoute() {
  CartExtension.PricingCartCalculator calculator=new OrchardCartCalculator();
  System.assertNotEquals(null,calculator);
  calculator.calculate(null);
  System.assertEquals(1,OrchardCartCalculator.calls);
  System.assertEquals(null,OrchardCartHelper.itemAsCommerce(null));
  System.assertEquals(null,OrchardCartHelper.groupAsCommerce(null));
 }
 @IsTest static void directHelperRoute() {
  System.assertEquals(null,OrchardCartHelper.itemAsCommerce(null));
  System.assertEquals(null,OrchardCartHelper.groupAsCommerce(null));
 }
}`,
			}, nil)
			if result := sema.Analyze(index); index.HasErrors() || result.HasErrors() {
				t.Fatalf("native cart helper signatures accepted: %v %v", index.Diagnostics, result.Diagnostics)
			}
			for _, route := range []struct {
				name        string
				parallelism int
			}{{"isTest-serial", 1}, {"isTest-cached-parallel", 2}} {
				t.Run(route.name, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"OrchardCartProof"}, NoDiskCache: true, Parallelism: route.parallelism, ParallelMethods: route.parallelism > 1})
					if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
						t.Fatalf("cart project: %+v %+v", summary, run.Suites)
					}
					if run.Suites[0].Cases[0].Status != testreport.StatusPass {
						t.Fatal(run.Suites[0].Cases[0])
					}
				})
			}
		})
	}
}
