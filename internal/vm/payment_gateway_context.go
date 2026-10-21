package vm

import "strings"

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
