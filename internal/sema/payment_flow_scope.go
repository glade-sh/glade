package sema

import (
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// Give only captured method bodies a private view of these interface edges.
// The shared platform table and hydration cache remain at their base shape.
func semaPaymentFlowView(model *semaTypeMemberView, api string, capturedRoute bool) *semaTypeMemberView {
	if model == nil || !capturedRoute || (api != "62.0" && api != "67.0") {
		return model
	}
	result := *model
	result.current = make(map[string]typeMembers, len(model.current)+4)
	for key, value := range model.current {
		result.current[key] = value
	}
	result.methodFamilies = nil
	result.lastMethodFamilyValid = false
	for _, name := range []string{"commercepayments.SaleRequest", "commercepayments.AuthorizationReversalRequest", "commercepayments.SaleResponse", "commercepayments.AuthorizationReversalResponse"} {
		key := normalizeName(name)
		members, ok := model.lookup(key)
		if !ok || !members.platform {
			continue
		}
		members.interfaces = append(append([]string(nil), members.interfaces...), typesys.PaymentFlowCapturedInterface(name, api))
		result.current[key] = members
	}
	return &result
}

func semaPaymentFlowRoute(typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView) bool {
	return (typ.EffectiveAPIVersion == "62.0" || typ.EffectiveAPIVersion == "67.0") && (model.anonymous || (typ.IsTest && member.IsTest))
}

func semaPaymentFlowResponseAssignment(typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView, target, value string) bool {
	if !semaPaymentFlowRoute(typ, member, model) || !strings.EqualFold(target, "commercepayments.GatewayResponse") || !strings.EqualFold(typesys.PaymentFlowCapturedInterface(value, typ.EffectiveAPIVersion), target) {
		return false
	}
	targetMembers, ok := model.lookup(normalizeName(target))
	if !ok || !targetMembers.platform {
		return false
	}
	valueMembers, ok := model.lookup(normalizeName(value))
	return ok && valueMembers.platform
}

var paymentFlowContextDeclaration = regexp.MustCompile(`(?i)^\s*commercepayments\.PaymentGatewayContext\s+[a-z_$][a-z0-9_$]*\s*=\s*$`)

func semaPaymentFlowContextShape(typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView, name string, args []string, expr ir.Expr, source string, statementStart, constructorStart int) bool {
	if !semaPaymentFlowRoute(typ, member, model) || !strings.EqualFold(name, "commercepayments.PaymentGatewayContext") || len(args) != 2 || len(expr.Args) != 2 || len(expr.NamedArgs) != 0 || expr.Args[0].Kind != ir.ExprVariable || expr.Args[1].Kind != ir.ExprVariable {
		return false
	}
	if statementStart < 0 || constructorStart < statementStart || constructorStart > len(source) || !paymentFlowContextDeclaration.MatchString(source[statementStart:constructorStart]) || !simpleIdentifierPattern.MatchString(expr.Args[0].Name) {
		return false
	}
	request := strings.ToLower(args[0])
	expected := ""
	switch request {
	case "commercepayments.salerequest":
		expected = "commercepayments.RequestType.Sale"
	case "commercepayments.authorizationreversalrequest":
		expected = "commercepayments.RequestType.AuthorizationReversal"
	default:
		return false
	}
	if !strings.EqualFold(expr.Args[1].Name, expected) {
		return false
	}
	target, ok := model.lookup(normalizeName(name))
	if !ok || !target.platform {
		return false
	}
	requestType, ok := model.lookup(normalizeName(args[0]))
	return ok && requestType.platform
}

// Native rejects Decimal amount variables for these request signatures.
func semaPaymentDecimalConstructorMessage(typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView, name string, args []string, named int) string {
	if !semaPaymentFlowRoute(typ, member, model) || named != 0 || len(args) == 0 || !strings.EqualFold(args[0], "Decimal") {
		return ""
	}
	canonical := ""
	arity := 1
	switch strings.ToLower(name) {
	case "commercepayments.authorizationrequest":
		canonical = "commercepayments.AuthorizationRequest"
	case "commercepayments.salerequest":
		canonical = "commercepayments.SaleRequest"
	case "commercepayments.capturerequest":
		canonical, arity = "commercepayments.CaptureRequest", 2
	case "commercepayments.referencedrefundrequest":
		canonical, arity = "commercepayments.ReferencedRefundRequest", 2
	case "commercepayments.authorizationreversalrequest":
		canonical, arity = "commercepayments.AuthorizationReversalRequest", 2
	default:
		return ""
	}
	if len(args) != arity || (arity == 2 && !strings.EqualFold(args[1], "String")) {
		return ""
	}
	target, ok := model.lookup(normalizeName(name))
	if !ok || !target.platform {
		return ""
	}
	return "Constructor not defined: [" + canonical + "].<Constructor>(" + strings.Join(args, ", ") + ")"
}
