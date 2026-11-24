package typesys

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

// Owned named Apex observations at APIs 61, 62 and 67 establish these payment
// DTO inheritance and interface edges. The request interface's native delegate
// is opaque; this catalog does not provide a local payment service.
var paymentPlatformSymbolOverlays = []StandardSymbolSpec{
	{Name: "commercepayments.PaymentGatewayAdapter", Kind: apexast.DeclarationInterface, ReplaceConstructors: true},
	{Name: "commercepayments.PaymentGatewayRequest", Kind: apexast.DeclarationInterface},
	{Name: "commercepayments.GatewayResponse", Kind: apexast.DeclarationInterface},
	{Name: "commercepayments.BaseRequest", Interfaces: []string{"commercepayments.PaymentGatewayRequest"}},
	{Name: "commercepayments.AuthorizationRequest", SuperClass: "commercepayments.BaseRequest"},
	{Name: "commercepayments.CaptureRequest", SuperClass: "commercepayments.BaseRequest"},
	{Name: "commercepayments.RefundRequest", SuperClass: "commercepayments.BaseRequest"},
	{Name: "commercepayments.ReferencedRefundRequest", SuperClass: "commercepayments.RefundRequest"},
	{Name: "commercepayments.AuthorizationResponse", Interfaces: []string{"commercepayments.GatewayResponse"}},
	{Name: "commercepayments.CaptureResponse", Interfaces: []string{"commercepayments.GatewayResponse"}},
	{Name: "commercepayments.PostAuthorizationResponse", Interfaces: []string{"commercepayments.GatewayResponse"}},
	{Name: "commercepayments.ReferencedRefundResponse", Interfaces: []string{"commercepayments.GatewayResponse"}},
	{Name: "commercepayments.GatewayErrorResponse", Interfaces: []string{"commercepayments.GatewayResponse"}},
	{Name: "commercepayments.ReferencedRefundNotification", SuperClass: "commercepayments.BaseNotification"},
	{Name: "commercepayments.CaptureNotification", SuperClass: "commercepayments.BaseNotification"},
}

// PaymentFlowCapturedInterface is limited to the 62.0 and 67.0 source
// versions observed in payment_flow_shapes. Callers must also gate the route.
func PaymentFlowCapturedInterface(typeName, apiVersion string) string {
	if apiVersion != "62.0" && apiVersion != "67.0" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(typeName)) {
	case "commercepayments.salerequest", "commercepayments.authorizationreversalrequest":
		return "commercepayments.PaymentGatewayRequest"
	case "commercepayments.saleresponse", "commercepayments.authorizationreversalresponse":
		return "commercepayments.GatewayResponse"
	}
	return ""
}
