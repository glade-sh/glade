package vm

import "strings"

// These constructor shapes were observed in anonymous Apex at APIs 62 and 67.
// Linked user classes do not inherit this platform-only restriction.
func (vm *VM) paymentRequestOutsideTestError(typeName string, args []Value, namedArgs map[string]Value) error {
	if vm.testContext != nil || vm.currentClass != "" || vm.currentMethod.ClassName != "" || vm.currentTrigger ||
		(vm.currentMethod.APIVersion != "62.0" && vm.currentMethod.APIVersion != "67.0") ||
		len(namedArgs) != 0 || !hasTypePrefixFold(typeName, "commercepayments") {
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
	if !ok || len(args) != shape.arity {
		return nil
	}
	if class, found := vm.lookupClass(typeName); found && !class.PlatformType {
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
