package vm

import "testing"

// These controls also appear in the owned named native payment context probe.
func TestExecPaymentRequestControls(t *testing.T) {
	program, err := CompileAnonymous(`
Double amount=3.5;
commercepayments.AuthorizationRequest request=new commercepayments.AuthorizationRequest(amount);
System.assertEquals(3.5,request.amount);
request.currencyIsoCode='EUR';
System.assertEquals('EUR',request.currencyIsoCode);
Object boxed=request;
System.assert(!(boxed instanceof commercepayments.GatewayResponse));
Object response=new commercepayments.CaptureResponse();
System.assert(!(response instanceof commercepayments.PaymentGatewayRequest));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
