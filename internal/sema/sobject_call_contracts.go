package sema

import "strings"

// API62/API67 SObject overload cases reject an untyped
// single null argument between unrelated SObject member overloads. Typed-null
// selectors and the multi-argument addError overloads remain valid.
func semaAmbiguousSObjectNullCall(receiverType, method string, argTypes []string, model *semaTypeMemberView) bool {
	if len(argTypes) != 1 || !strings.EqualFold(argTypes[0], "null") || !isSemaSObjectLike(receiverType, model) {
		return false
	}
	switch normalizeName(method) {
	case "get", "isset", "getsobject", "getsobjects", "adderror":
		return true
	}
	return false
}
