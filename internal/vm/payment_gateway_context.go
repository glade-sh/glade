package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// These same-type declarations are covered by payment_flow_shapes/org.tsv.
func (vm *VM) paymentRequestDeclarationError(inst ir.Instruction) error {
	e := inst.Expr
	name, constructor := strings.CutPrefix(e.Callee, "new:")
	if !constructor || !hasTypePrefixFold(name, "commercepayments") || !strings.EqualFold(inst.Type, name) {
		return nil
	}
	switch vm.paymentRequestAmountType(e) {
	case "double", "integer":
		return vm.paymentRequestOutsideTestError(name, len(e.Args))
	}
	if typesys.PaymentFlowCapturedInterface(name, vm.currentMethod.APIVersion) != "commercepayments.PaymentGatewayRequest" ||
		len(e.Args) == 0 || len(e.NamedArgs) != 0 {
		return nil
	}
	if len(e.Args) == 2 && (e.Args[1].Kind != ir.ExprLiteral || vm.declaredExpressionType(e.Args[1]) != "String") {
		return nil
	}
	amount := e.Args[0]
	if amount.Kind != ir.ExprLiteral &&
		!(amount.Kind == ir.ExprCall && strings.EqualFold(amount.Callee, "__cast:Double") && len(amount.Args) == 1 && amount.Args[0].Kind == ir.ExprLiteral) {
		return nil
	}
	switch vm.declaredExpressionType(amount) {
	case "Double", "Integer":
		return vm.paymentRequestOutsideTestError(name, len(e.Args))
	}
	return nil
}

// The authorization context fixture passes a directly constructed request as
// its first argument (payment_gateway_adapter/context/org.tsv).
func (vm *VM) paymentAuthorizationContextError(e ir.Expr) error {
	const context = "commercepayments.PaymentGatewayContext"
	const request = "commercepayments.AuthorizationRequest"
	if !strings.EqualFold(e.Callee, "new:"+context) || len(e.Args) != 2 || len(e.NamedArgs) != 0 {
		return nil
	}
	child, kind := e.Args[0], e.Args[1]
	if !strings.EqualFold(child.Callee, "new:"+request) || len(child.Args) != 1 ||
		kind.Kind != ir.ExprVariable || !strings.EqualFold(kind.Name, "commercepayments.RequestType.Authorize") ||
		vm.paymentRequestAmountType(child) != "double" {
		return nil
	}
	if class, found := vm.lookupClassRaw(context); found && !class.PlatformType {
		return nil
	}
	return vm.paymentRequestOutsideTestError(request, len(child.Args))
}

func (vm *VM) paymentRequestAmountType(e ir.Expr) string {
	if e.Kind != ir.ExprCall || e.Left != nil || len(e.NamedArgs) != 0 || len(e.Args) == 0 {
		return ""
	}
	for _, arg := range e.Args {
		if arg.Kind != ir.ExprVariable || arg.Name == "" || strings.Contains(arg.Name, ".") {
			return ""
		}
	}
	name, found := vm.lookupGlobalName(e.Args[0].Name)
	if !found {
		return ""
	}
	return strings.ToLower(canonicalRuntimePlatformType(vm.VarTypes[name]))
}

// These constructor shapes were observed in anonymous Apex at APIs 62 and 67.
// Linked user classes do not inherit this platform-only restriction.
func (vm *VM) paymentRequestOutsideTestError(typeName string, arity int) error {
	if vm.testContext != nil || vm.currentClass != "" || vm.currentMethod.ClassName != "" || vm.currentTrigger ||
		(vm.currentMethod.APIVersion != "62.0" && vm.currentMethod.APIVersion != "67.0") {
		return nil
	}
	names := map[string]struct {
		canonical string
		arity     int
	}{
		"commercepayments.authorizationrequest":         {"commercepayments.AuthorizationRequest", 1},
		"commercepayments.capturerequest":               {"commercepayments.CaptureRequest", 2},
		"commercepayments.salerequest":                  {"commercepayments.SaleRequest", 1},
		"commercepayments.referencedrefundrequest":      {"commercepayments.ReferencedRefundRequest", 2},
		"commercepayments.authorizationreversalrequest": {"commercepayments.AuthorizationReversalRequest", 2},
	}
	shape, ok := names[strings.ToLower(typeName)]
	if !ok || arity != shape.arity {
		return nil
	}
	if class, found := vm.lookupClassRaw(typeName); found && !class.PlatformType {
		return nil
	}
	return newExceptionError("System.UnsupportedOperationException", shape.canonical+" cannot be constructed directly outside of a test")
}

// Native payment constructors expose some arguments under different property
// names. Retain the supplied identities rather than creating default DTOs.
func (vm *VM) bindPassivePaymentConstructor(object *Value, ctor Method, args []Value) {
	if !ctor.IsConstructor || len(ctor.Program.Instructions) != 0 {
		return
	}
	if class, found := vm.lookupClass(ctor.ClassName); found && !class.PlatformType {
		return
	}
	switch strings.ToLower(ctor.ClassName) {
	case "commercepayments.paymentgatewaycontext":
		if len(args) == 2 {
			object.Fields["paymentRequest"] = args[0]
			object.Fields["paymentRequestType"] = args[1]
		}
	case "commercepayments.capturerequest", "commercepayments.authorizationreversalrequest":
		if len(args) == 2 {
			object.Fields["paymentAuthorizationId"] = args[1]
		}
	case "commercepayments.authapipaymentmethodrequest":
		if len(args) == 1 {
			object.Fields["cardPaymentMethod"] = args[0]
		}
	}
}
