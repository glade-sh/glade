package sema

import "strings"

// The captured row captures Math.abs(Account) rejection text on both routes at
// API 62/67/68. Apply only after overload matching has rejected this call.
// Other Math methods, argument types and collection diagnostics stay unchanged.
func capturedMathAbsAccountNativeMessage(receiverType, method string, argTypes []string, receiverMode string, model *semaTypeMemberView) string {
	if receiverMode != "class" || !strings.EqualFold(receiverType, "Math") ||
		!strings.EqualFold(method, "abs") || len(argTypes) != 1 || !strings.EqualFold(argTypes[0], "Account") {
		return ""
	}
	owner, _, known := semaLookupTypeMembers(model, receiverType)
	if !known || !owner.platform {
		return ""
	}
	argument, _, known := semaLookupTypeMembers(model, argTypes[0])
	if !known || !argument.sobject {
		return ""
	}
	return "Method does not exist or incorrect signature: void abs(Account) from the type System.Math"
}
