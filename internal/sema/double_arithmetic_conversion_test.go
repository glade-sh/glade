package sema

import "testing"

// Native source61/62/67 accepts these Double expressions, including where the
// call must choose Decimal.valueOf(Double) rather than a Decimal argument.
func TestAnalyzeDoubleArithmeticDecimalConversion(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		for _, expression := range []string{"amount+2", "amount-2", "amount*2", "amount/2", "amount/whole", "amount/divisor", "2/amount", "(amount+2)/2"} {
			t.Run(api+"/"+expression, func(t *testing.T) {
				source := "Double amount=13.25;Long whole=2L;Double divisor=2.5;Decimal result=Decimal.valueOf(" + expression + ").setScale(2);"
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"PromotionConsumer.cls": "public class PromotionConsumer {public static void convert(){" + source + "}}"}, api)
				if result.HasErrors() {
					t.Fatalf("native-accepted arithmetic rejected: %#v", result.Diagnostics)
				}
			})
		}
	}
}
