package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// These interface edges were captured in anonymous Apex and direct test
// methods. Source classes and other API versions keep the base hierarchy.
func (vm *VM) paymentFlowCapturedInterface(name string) string {
	if vm == nil || vm.currentTrigger {
		return ""
	}
	version := vm.currentSourceExecutionPolicy().APIVersion
	edge := typesys.PaymentFlowCapturedInterface(name, version)
	if edge == "" {
		return ""
	}
	if class, ok := vm.lookupClass(name); ok && !class.PlatformType {
		return ""
	}
	if vm.testContext == nil {
		if vm.currentClass == "" && vm.currentMethod.ClassName == "" {
			return edge
		}
		if vm.currentMethod.anonymousLocal && edge == "commercepayments.GatewayResponse" {
			return edge
		}
	}
	className := vm.currentMethod.ClassName
	if className == "" {
		className = vm.currentClass
	}
	class, ok := vm.lookupClass(className)
	if vm.testContext != nil && ok && class.IsTest && (methodHasModifier(vm.currentMethod.Modifiers, "istest") || methodHasModifier(vm.currentMethod.Modifiers, "testmethod")) {
		return edge
	}
	return ""
}

// Convert just the constructor argument. The passive context stores the
// interface view so getPaymentRequest can return it with its concrete identity.
func (vm *VM) convertPaymentFlowContextRequest(args []Value) {
	const context = "commercepayments.PaymentGatewayContext"
	const requestInterface = "commercepayments.PaymentGatewayRequest"
	if vm.testContext == nil {
		return
	}
	if class, ok := vm.lookupClassRaw(context); ok && !class.PlatformType {
		return
	}
	request := args[0].Type
	if !strings.EqualFold(vm.paymentFlowCapturedInterface(request), requestInterface) {
		return
	}
	var want string
	switch strings.ToLower(request) {
	case "commercepayments.salerequest":
		want = "Sale"
	case "commercepayments.authorizationreversalrequest":
		want = "AuthorizationReversal"
	default:
		return
	}
	if !strings.EqualFold(args[1].Type, "commercepayments.RequestType") || !strings.EqualFold(args[1].Text, want) {
		return
	}
	if class, ok := vm.lookupClassRaw(requestInterface); ok && !class.PlatformType {
		return
	}
	args[0].Static = requestInterface
}

// Only a direct local declaration with the recorded variable and enum shapes
// may use the temporary constructor conversion.
func paymentFlowContextDeclaration(inst ir.Instruction) bool {
	e := inst.Expr
	if !strings.EqualFold(inst.Type, "commercepayments.PaymentGatewayContext") || e.Kind != ir.ExprCall || !strings.EqualFold(e.Callee, "new:commercepayments.PaymentGatewayContext") || len(e.Args) != 2 || len(e.NamedArgs) != 0 {
		return false
	}
	if e.Args[0].Kind != ir.ExprVariable || e.Args[0].Name == "" || strings.Contains(e.Args[0].Name, ".") || e.Args[1].Kind != ir.ExprVariable {
		return false
	}
	return strings.EqualFold(e.Args[1].Name, "commercepayments.RequestType.Sale") || strings.EqualFold(e.Args[1].Name, "commercepayments.RequestType.AuthorizationReversal")
}
