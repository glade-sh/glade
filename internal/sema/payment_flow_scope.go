package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Give only captured method bodies a private view of these interface edges.
// The shared platform table and hydration cache remain at their base shape.
func semaPaymentFlowView(model *semaTypeMemberView, api string, capturedRoute bool) *semaTypeMemberView {
	if model == nil || !capturedRoute || !typesys.PaymentFlowCapturedVersion(api) {
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
		edge := typesys.PaymentFlowCapturedInterface(name, api)
		target, ok := model.lookup(normalizeName(edge))
		if !ok || !target.platform {
			continue
		}
		members.interfaces = append(append([]string(nil), members.interfaces...), edge)
		result.current[key] = members
	}
	return &result
}

func semaPaymentFlowRoute(typ typesys.TypeSymbol, member typesys.MemberSymbol, model *semaTypeMemberView) bool {
	return (model.anonymous || (typ.IsTest && member.IsTest)) && typesys.PaymentFlowCapturedVersion(typ.EffectiveAPIVersion)
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

func (a *Analyzer) semaPaymentFlowContextInitializer(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, model *semaTypeMemberView) bool {
	const context = "commercepayments.PaymentGatewayContext"
	expr := inst.Expr
	if inst.Op != ir.OpDeclare || !strings.EqualFold(inst.Type, context) ||
		expr.Kind != ir.ExprCall || !strings.EqualFold(expr.Callee, "new:"+context) ||
		len(expr.Args) != 2 || len(expr.NamedArgs) != 0 || expr.Args[0].Kind != ir.ExprVariable ||
		!simpleIdentifierPattern.MatchString(expr.Args[0].Name) || expr.Args[1].Kind != ir.ExprVariable ||
		!semaPaymentFlowRoute(typ, member, model) {
		return false
	}
	if model.anonymous {
		// Anonymous execution stops at the captured request declaration.
		// If the request can reach the context constructor, the VM needs
		// a constructor available on the base platform hierarchy.
		request, ok := scope.binding(expr.Args[0].Name)
		if model.anonymousLocalMethod || !ok || !request.paymentRequestRejected {
			return false
		}
	}
	args := irCallArgTypes(a, expr.Args, scope, model, typ.Name)
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
	target, ok := model.lookup(normalizeName(context))
	if !ok || !target.platform {
		return false
	}
	requestType, ok := model.lookup(normalizeName(args[0]))
	return ok && requestType.platform
}

func (a *Analyzer) semaPaymentRequestDeclarationOutsideTest(typ typesys.TypeSymbol, inst ir.Instruction, scope irSemaScope, model *semaTypeMemberView) bool {
	if !strings.HasPrefix(strings.ToLower(inst.Expr.Callee), "new:commercepayments.") ||
		typesys.PaymentFlowCapturedInterface(inst.Type, typ.EffectiveAPIVersion) != "commercepayments.PaymentGatewayRequest" {
		return false
	}
	target, ok := model.lookup(normalizeName(inst.Type))
	if !ok || !target.platform {
		return false
	}
	return vm.PaymentRequestDeclarationOutsideTestShape(inst, typ.EffectiveAPIVersion, func(arg ir.Expr) string {
		if arg.Kind == ir.ExprVariable {
			declared, _ := scope.lookup(arg.Name)
			return declared
		}
		return a.inferIRExprType(arg, scope, model, typ.Name)
	})
}

// The literal context probes declare the concrete request before passing it
// to the context. A Decimal literal fails at that first declaration.
func (a *Analyzer) semaPaymentFlowDecimalLiteralInitializer(typ typesys.TypeSymbol, member typesys.MemberSymbol, inst ir.Instruction, scope irSemaScope, model *semaTypeMemberView) string {
	expr := inst.Expr
	name, constructor := strings.CutPrefix(expr.Callee, "new:")
	if !constructor || !strings.EqualFold(name, inst.Type) || len(expr.Args) == 0 ||
		expr.Args[0].Kind != ir.ExprLiteral ||
		model.anonymousLocalMethod || (!model.anonymous && !hasModifier(member.Modifiers, "istest")) {
		return ""
	}
	resolved := resolveNestedTypeReference(model, typ.Name, name)
	if typesys.PaymentFlowCapturedInterface(resolved, typ.EffectiveAPIVersion) != "commercepayments.PaymentGatewayRequest" {
		return ""
	}
	for _, arg := range expr.Args[1:] {
		if arg.Kind != ir.ExprLiteral {
			return ""
		}
	}
	return semaPaymentDecimalConstructorMessage(typ, member, model, resolved, irCallArgTypes(a, expr.Args, scope, model, typ.Name), len(expr.NamedArgs))
}

// Native rejects Decimal amounts for these request signatures.
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
