package vm

import (
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
	"strings"
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
	if vm.currentClass == "" && vm.currentMethod.ClassName == "" && vm.testContext == nil {
		return edge
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

func (vm *VM) paymentFlowContextRequest(typeName string, args []Value, namedArgs map[string]Value) string {
	if vm.testContext == nil || !vm.paymentFlowContextDeclaration || !strings.EqualFold(typeName, "commercepayments.PaymentGatewayContext") || len(args) != 2 || len(namedArgs) != 0 {
		return ""
	}
	if class, ok := vm.lookupClass(typeName); ok && !class.PlatformType {
		return ""
	}
	request := args[0].Type
	if !strings.EqualFold(vm.paymentFlowCapturedInterface(request), "commercepayments.PaymentGatewayRequest") {
		return ""
	}
	want := ""
	switch strings.ToLower(request) {
	case "commercepayments.salerequest":
		want = "Sale"
	case "commercepayments.authorizationreversalrequest":
		want = "AuthorizationReversal"
	default:
		return ""
	}
	if !strings.EqualFold(args[1].Type, "commercepayments.RequestType") || !strings.EqualFold(args[1].Text, want) {
		return ""
	}
	return request
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
