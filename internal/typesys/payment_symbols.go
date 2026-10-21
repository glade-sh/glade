package typesys

import "github.com/glade-sh/glade/internal/apexast"

// Owned named Apex observations at APIs 61, 62 and 67 establish these payment
// DTO inheritance and interface edges. The request interface's native delegate
// is opaque; this catalog does not provide a local payment service.
var paymentPlatformSymbolOverlays = []StandardSymbolSpec{
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
