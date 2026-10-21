package sema

import "github.com/glade-sh/glade/internal/typesys"

// Exception's public accessors remain available through an intermediate custom
// exception that does not redeclare the accessor.
func hasInheritedExceptionMethodSignature(model *semaTypeMemberView, typ typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if hasModifier(member.Modifiers, "static") || !hasPlatformInheritedMethodSignature(typesys.TypeSymbol{SuperClass: "Exception"}, member) {
		return false
	}
	seen := make(map[string]bool)
	for current := resolveNestedTypeName(model, typ.Name, typ.SuperClass); current != ""; {
		key := normalizeName(current)
		if seen[key] {
			return false
		}
		seen[key] = true
		if key == "exception" || key == "system.exception" {
			return !semaProjectTypeShadowsPlatform(model, current)
		}
		parent, _, ok := semaLookupTypeMembers(model, current)
		if !ok {
			return false
		}
		for _, candidate := range parent.methods[normalizeName(member.Name)] {
			if sameInheritedSemaSignature(model, parent.name, candidate, typ.Name, member) {
				return false
			}
		}
		current = resolveNestedTypeName(model, parent.name, parent.superClass)
	}
	return false
}
