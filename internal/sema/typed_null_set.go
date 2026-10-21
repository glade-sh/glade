package sema

import "github.com/glade-sh/glade/internal/apexast"

// The captured rows: Set<UserClass> is not an argument for the captured
// Set<String>/Set<Object>/Set<Id> overload set. Do not change assignments,
// lone Set<Object> methods, other overload sets or List/Map covariance here.
func capturedUserClassSetOverloadRejection(candidates []resolvedMember, argTypes []string, model *semaTypeMemberView) bool {
	if len(argTypes) != 1 || capturedCollectionOverloadBase(candidates) != "set" {
		return false
	}
	base, args := semaGenericBaseAndArgs(semaCanonicalAssignableType(argTypes[0]))
	if normalizeName(base) != "set" || len(args) != 1 {
		return false
	}
	for _, scalar := range []string{"String", "Object", "Id"} {
		if semaProjectTypeShadowsPlatform(model, scalar) {
			return false
		}
	}
	member, _, known := semaLookupTypeMembers(model, args[0])
	return known && member.kind == apexast.DeclarationClass && !member.platform && !member.sobject
}
