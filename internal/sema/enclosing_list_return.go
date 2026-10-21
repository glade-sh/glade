package sema

import "strings"

// The captured rows capture an implicit call from a nested type to its
// immediate outer private static Integer[,Integer],List<Object> helper.
// Direct candidates, other signatures and deeper lexical searches stay on
// their existing inference path; in particular the local scalar twin wins.
func enclosingListHelperReturnType(model *semaTypeMemberView, receiver, method, currentType, receiverMode string, direct []resolvedMember, argTypes []string) string {
	if receiverMode != "implicit" || !strings.EqualFold(receiver, currentType) || len(direct) != 0 || (len(argTypes) != 2 && len(argTypes) != 3) {
		return ""
	}
	dot := strings.LastIndexByte(currentType, '.')
	if dot <= 0 {
		return ""
	}
	for _, arg := range argTypes[:len(argTypes)-1] {
		if !strings.EqualFold(arg, "Integer") {
			return ""
		}
	}
	base, elements := semaGenericBaseAndArgs(argTypes[len(argTypes)-1])
	if !strings.EqualFold(base, "List") || len(elements) != 1 || !strings.EqualFold(elements[0], "String") {
		return ""
	}
	selected, ok, ambiguous := bestResolvedMemberByArgTypes(resolveImplicitMemberMethods(model, receiver, method), argTypes, model)
	if !ok || ambiguous || !strings.EqualFold(selected.owner, currentType[:dot]) ||
		!hasModifier(selected.member.Modifiers, "private") || !hasModifier(selected.member.Modifiers, "static") ||
		len(selected.member.Parameters) != len(argTypes) || semaResolvedMemberReturnType(model, selected) != "List<Object>" {
		return ""
	}
	for _, param := range selected.member.Parameters[:len(argTypes)-1] {
		if !strings.EqualFold(param.Type, "Integer") {
			return ""
		}
	}
	base, elements = semaGenericBaseAndArgs(selected.member.Parameters[len(argTypes)-1].Type)
	if !strings.EqualFold(base, "List") || len(elements) != 1 || !strings.EqualFold(elements[0], "Object") {
		return ""
	}
	return "List<Object>"
}
