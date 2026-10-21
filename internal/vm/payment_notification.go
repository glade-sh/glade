package vm

// A populated notification without hosted gateway context returns a failed
// result in named tests. It does not create payment records or logs.
func (vm *VM) paymentNotificationMissingContextResult(args []Value) (Value, bool) {
	if vm.testContext == nil || len(args) != 1 || args[0].Kind != ValueObject {
		return Null, false
	}
	notification := args[0]
	amount, reference := notification.Fields["amount"], notification.Fields["gatewayReferenceNumber"]
	if amount.Kind == "" || amount.Kind == ValueNull || reference.Kind != ValueString || reference.Text == "" {
		return Null, false
	}
	if provider := notification.Fields["gatewayProviderId"]; provider.Kind != "" && provider.Kind != ValueNull {
		return Null, false
	}
	result := Object("commercepayments.NotificationSaveResult")
	result.Fields["statusCode"] = Int(400)
	result.Fields["success"] = Bool(false)
	result.Fields["errorMessage"] = String("No records found for given gatewayProviderId null and gatewayReferenceNumber " + reference.Text)
	return result, true
}
