package sema

import "strings"

// A non-type suffix on a class literal names its nearest enclosing type.
// Static values, including enum constants, are invalid type references.
func semaClassLiteralPrefix(model *semaTypeMemberView, owner, name string) (string, bool) {
	if _, args := semaGenericBaseAndArgs(name); len(args) != 0 {
		return "", false
	}
	if _, _, ok := semaLookupTypeMembers(model, resolveNestedTypeName(model, owner, name)); ok {
		return "", false
	}
	for dot := strings.LastIndexByte(name, '.'); dot > 0; dot = strings.LastIndexByte(name[:dot], '.') {
		prefix := resolveNestedTypeName(model, owner, name[:dot])
		members, _, ok := semaLookupTypeMembers(model, prefix)
		if !ok {
			continue
		}
		if members.platform && !members.sobject {
			return "", false
		}
		next, _, _ := strings.Cut(name[dot+1:], ".")
		if field, ok := semaResolveField(model, prefix, next, make(map[string]bool)); ok && hasModifier(field.member.Modifiers, "static") {
			return "", true
		}
		return prefix, false
	}
	return "", false
}
