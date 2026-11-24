package vm

import (
	"strings"
	"testing"
)

func TestPaymentRequestConstructorCapturedScope(t *testing.T) {
	requests := []string{
		"commercepayments.AuthorizationRequest(3.5)",
		"commercepayments.CaptureRequest(3.5, 'ref')",
		"commercepayments.SaleRequest(3.5)",
		"commercepayments.ReferencedRefundRequest(3.5, 'ref')",
		"commercepayments.AuthorizationReversalRequest(3.5, 'ref')",
	}
	for _, api := range []string{"61.0", "62.0", "63.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "named", "test"} {
			for _, request := range requests {
				t.Run(api+"/"+route+"/"+request, func(t *testing.T) {
					machine := New(nil)
					body, err := CompileAnonymousWithOptions("Object request = new "+request+";", CompileOptions{APIVersion: api})
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
