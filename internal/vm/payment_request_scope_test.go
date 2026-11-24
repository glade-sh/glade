package vm

import (
	"strings"
	"testing"
)

func TestPaymentRequestConstructorCapturedScope(t *testing.T) {
	requests := []string{
		"commercepayments.AuthorizationRequest(amount)",
		"commercepayments.CaptureRequest(amount, ref)",
		"commercepayments.SaleRequest(amount)",
		"commercepayments.ReferencedRefundRequest(amount, ref)",
		"commercepayments.AuthorizationReversalRequest(amount, ref)",
	}
	for _, api := range []string{"61.0", "62.0", "63.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "named", "test"} {
			for _, request := range requests {
				t.Run(api+"/"+route+"/"+request, func(t *testing.T) {
					machine := New(nil)
					requestType := strings.Split(request, "(")[0]
					body, err := CompileAnonymousWithOptions("Double amount=3.5; String ref='ref'; "+requestType+" request = new "+request+";", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if route == "test" {
						machine.EnableTestContext()
					}
					if route == "named" {
						method := Method{Name: "PaymentCaller.run", ClassName: "PaymentCaller", APIVersion: api, IsStatic: true, ReturnType: "void", Access: "public", Program: body}
						if err := machine.RegisterClass(Class{Name: "PaymentCaller", APIVersion: api, Methods: map[string]Method{"run": method}}); err != nil {
							t.Fatal(err)
						}
						_, err = machine.CallStatic("PaymentCaller.run", nil)
					} else {
						_, err = machine.Execute(body)
					}
					wantException := route == "anonymous" && (api == "62.0" || api == "67.0")
					if wantException {
						if err == nil || !strings.Contains(err.Error(), "System.UnsupportedOperationException") {
							t.Fatalf("expected captured exception; got %v", err)
						}
					} else if err != nil {
						t.Fatalf("uncaptured route or version changed: %v", err)
					}
				})
			}
		}
	}
}

// These uncaptured declarations retain the base behavior.
func TestPaymentRequestUncapturedDeclarationRemainsUnchanged(t *testing.T) {
	shapes := []string{
		"Object request = new commercepayments.SaleRequest(3.5);",
		"Double amount=3.5; Object request = new commercepayments.SaleRequest(amount);",
		"commercepayments.SaleRequest request = new commercepayments.SaleRequest(3.5);",
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, source := range shapes {
			t.Run(api+"/"+source, func(t *testing.T) {
				body, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(body); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
