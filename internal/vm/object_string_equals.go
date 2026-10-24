package vm

import "strings"

// OSE01/OSE02: Object-held String values use case-insensitive Object.equals,
// including when called from a user equals(Object) implementation. Explicit
// String.equals stays case-sensitive. No other Object-value family is inferred.
func (vm *VM) objectViewStringEquals(receiverName string, receiver Value, method string, args []Value) (Value, bool) {
	if !strings.EqualFold(method, "equals") || len(args) != 1 ||
		receiver.Kind != ValueString || args[0].Kind != ValueString ||
		(receiver.Type != "" && !strings.EqualFold(receiver.Type, "String")) ||
		(args[0].Type != "" && !strings.EqualFold(args[0].Type, "String")) {
		return Null, false
	}
	view := vm.declaredReceiverType(receiverName)
	if view == "" {
		view = receiver.Static
	}
	if !strings.EqualFold(view, "Object") && !strings.EqualFold(view, "System.Object") {
		return Null, false
	}
	if looksLikeComparableIDText(receiver.Text) && looksLikeComparableIDText(args[0].Text) && apexIDTextEqual(receiver.Text, args[0].Text) {
		return Bool(true), true
	}
	return Bool(strings.EqualFold(receiver.Text, args[0].Text)), true
}
