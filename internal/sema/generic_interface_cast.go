package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaGenericInterfaceCastParts(typeName string, model *semaTypeMemberView) (string, []string, bool) {
	base, args := semaGenericBaseAndArgs(semaCanonicalPlatformAlias(typeName))
	members, _, ok := semaLookupTypeMembers(model, base)
	if !ok || !members.platform || members.kind != apexast.DeclarationInterface {
		return "", nil, false
	}
	return strings.ToLower(base), args, true
}

// Native check-only declarations at API 62/65/67/68 accept casts between
// parameterized platform interfaces and user classes or interfaces, including
// concrete classes that do not implement the interface. Runtime casts still
// check the actual value. Collections and instanceof retain their own rules.
func semaParameterizedInterfaceCastCompatible(owner, target, value string, model *semaTypeMemberView) bool {
	target = resolveNestedTypeReference(model, owner, target)
	value = resolveNestedTypeReference(model, owner, value)
	compatible := func(iface, other string) bool {
		_, args, ok := semaGenericInterfaceCastParts(iface, model)
		if !ok || len(args) == 0 {
			return false
		}
		members, _, found := semaLookupTypeMembers(model, other)
		return found && !members.platform && !members.sobject &&
			(members.kind == apexast.DeclarationClass || members.kind == apexast.DeclarationInterface)
	}
	return compatible(target, value) || compatible(value, target)
}

// GI012/018/024/030 and GI036/040 reject the captured cast type spellings.
// The native wording is evidence only; local diagnostics remain independent.
func semaGenericInterfaceCastTypeError(target string, model *semaTypeMemberView) string {
	base, args, ok := semaGenericInterfaceCastParts(target, model)
	if !ok {
		return ""
	}
	switch base {
	case "comparator", "database.batchable", "iterable", "iterator":
		if len(args) == 0 {
			// GI012/018/024/030 capture these spellings exactly. In
			// particular, GI024/030 do not capture System.Iterable or
			// System.Iterator; preserve those documented alias neighbors.
			switch normalizeName(target) {
			case "system.comparator", "database.batchable", "iterable", "iterator":
				return "generic interface cast requires a type argument"
			}
		}
	case "comparable", "schedulable":
		if len(args) == 1 && strings.EqualFold(args[0], "Object") {
			return "non-generic interface cast cannot use an Object type argument"
		}
	}
	return ""
}

// GI053 rejects direct initialization of a Comparator<Object> local with a
// new Comparator<String> implementation. Cast results, method parameters,
// returns, fields, other argument pairs and inherited implementations retain
// their existing checks until native observations cover those shapes.
func semaGenericInterfaceDirectAssignmentRejected(target, value string, expr ir.Expr, model *semaTypeMemberView) bool {
	if expr.Kind != ir.ExprCall || !strings.HasPrefix(expr.Callee, "new:") || len(expr.Args) != 0 || len(expr.NamedArgs) != 0 {
		return false
	}
	base, args, ok := semaGenericInterfaceCastParts(target, model)
	if !ok || base != "comparator" || len(args) != 1 || !strings.EqualFold(args[0], "Object") {
		return false
	}
	members, _, ok := semaLookupTypeMembers(model, value)
	if !ok || members.platform || members.kind != apexast.DeclarationClass {
		return false
	}
	for _, iface := range members.interfaces {
		base, args, ok := semaGenericInterfaceCastParts(iface, model)
		if ok && base == "comparator" && len(args) == 1 && strings.EqualFold(args[0], "String") {
			return true
		}
	}
	return false
}

// GI047 rejects the deployed inner Batchable<SObject> subject on both routes
// at API 62/67/68. GI059 accepts inner Schedulable; other Batchable arguments
// and inherited interface declarations have no subject row in this packet.
func semaGenericInterfaceInnerBatchRejected(typ typesys.TypeSymbol, model *semaTypeMemberView) bool {
	if typ.Kind != apexast.DeclarationClass || typ.NestingDepth != 1 {
		return false
	}
	for _, iface := range typ.Interfaces {
		base, args, ok := semaGenericInterfaceCastParts(iface, model)
		if ok && base == "database.batchable" && len(args) == 1 && strings.EqualFold(args[0], "SObject") {
			return true
		}
	}
	return false
}
