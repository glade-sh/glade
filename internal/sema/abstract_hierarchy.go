package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// AB_CHAIN/DEPTH/INTERFACE/VIRTUAL and their rejection twins capture named,
// zero-argument void redeclarations. AB_INNER_PARAM and AB_INNER_PARAM_PLAIN
// capture Object methods with one enclosing sibling-class parameter. Both
// routes agree at APIs 62/67/68; other declaration shapes keep their checks.
func capturedAbstractHierarchyDiagnostics(index typesys.Index, typ typesys.TypeSymbol, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if typ.Dependency || typ.Namespace != "" || typ.Kind != apexast.DeclarationClass || typ.SuperClass == "" ||
		hasModifier(typ.Modifiers, vm.AnonymousClassModifier) ||
		!typeUsesAPIVersionAtLeast(typ, 62) || typeUsesAPIVersionAtLeast(typ, 69) {
		return diagnostics
	}
	var model *semaTypeMemberView
	for _, member := range typ.Members {
		if member.Kind != apexast.DeclarationMethod || !hasModifier(member.Modifiers, "override") ||
			hasModifier(member.Modifiers, "static") || member.HasBody && hasModifier(member.Modifiers, "abstract") ||
			hasModifier(member.Modifiers, "final") || hasModifier(member.Modifiers, "abstract") && hasModifier(member.Modifiers, "virtual") ||
			(!hasModifier(member.Modifiers, "public") && !hasModifier(member.Modifiers, "protected")) {
			continue
		}
		zeroArgument := typ.OwnerName == "" && hasModifier(typ.Modifiers, "public") && len(member.Parameters) == 0 &&
			(strings.EqualFold(member.Type, "void") || hasModifier(member.Modifiers, "abstract") && strings.EqualFold(member.Type, "Integer"))
		innerParameter := typ.OwnerName != "" && len(member.Parameters) == 1 && strings.EqualFold(member.Type, "Object") &&
			hasModifier(member.Modifiers, "public") && hasModifier(member.Modifiers, "abstract")
		if !zeroArgument && !innerParameter {
			continue
		}
		if model == nil {
			model = buildSemaTypeMemberView(index)
		}
		parentName := resolveNestedTypeName(model, typ.Name, typ.SuperClass)
		parent, _, ok := semaLookupTypeMembers(model, parentName)
		if !ok || parent.dependency || parent.platform || parent.namespace != "" {
			continue
		}
		if innerParameter {
			// The captured parameter is a sibling class, resolved in each
			// declaration's scope; its spelling and parameter name need not match.
			parameter := resolvedLifecycleMethod(model, typ.Name, member).Parameters[0].Type
			parameterType, _, found := semaLookupTypeMembers(model, parameter)
			if !found || parameterType.kind != apexast.DeclarationClass || parameterType.dependency ||
				!strings.EqualFold(strings.TrimSuffix(parameterType.name, "."+shortNestedTypeName(parameterType.name)), typ.OwnerName) ||
				!strings.EqualFold(strings.TrimSuffix(parent.name, "."+shortNestedTypeName(parent.name)), typ.OwnerName) ||
				!hasModifier(parent.modifiers, "abstract") {
				continue
			}
		}
		inherited, owner, found := abstractHierarchyNearestMethod(model, typ.Name, parentName, member)
		if !found {
			// AB_NO_ANCESTOR: the abstract parent declares another public
			// zero-argument void method; the override names no ancestor method.
			if !zeroArgument || !hasModifier(member.Modifiers, "abstract") || !abstractHierarchyVoidParent(parent) {
				continue
			}
		} else if !hasModifier(inherited.Modifiers, "public") ||
			zeroArgument && !strings.EqualFold(inherited.Type, "void") ||
			innerParameter && (!strings.EqualFold(inherited.Type, "Object") || !hasModifier(inherited.Modifiers, "abstract") ||
				!strings.EqualFold(strings.TrimSuffix(owner.name, "."+shortNestedTypeName(owner.name)), typ.OwnerName)) {
			continue
		}
		abstract := hasModifier(member.Modifiers, "abstract")
		if abstract && !hasModifier(typ.Modifiers, "abstract") {
			continue
		}
		if !abstract && (!found || !hasModifier(inherited.Modifiers, "abstract") || !hasModifier(inherited.Modifiers, "override")) {
			continue
		}
		native := ""
		if !found {
			native = "@Override specified for non-overriding method: " + semaNativeOverrideSignature(model, typ.Name, member)
		} else if strings.EqualFold(member.Type, "Integer") && strings.EqualFold(inherited.Type, "void") {
			native = "Method return types clash: Integer vs void from the type " + typ.Name
		} else if prefix := abstractHierarchyInvalidOverride(model, owner, inherited, make(map[string]bool)); prefix != "" {
			native = prefix + semaNativeOverrideSignature(model, typ.Name, member)
		} else if !hasModifier(inherited.Modifiers, "abstract") && !hasModifier(inherited.Modifiers, "virtual") {
			native = "Non-virtual, non-abstract methods cannot be overridden: " + semaNativeOverrideSignature(model, typ.Name, member)
		}
		if abstract {
			out := diagnostics[:0]
			for _, d := range diagnostics {
				if d.Message == fmt.Sprintf("method %q cannot be both abstract and override", member.Name) &&
					d.Range != nil && d.Range.Start.Offset == member.Range.Start.Offset {
					if native == "" {
						continue
					}
					d.NativeMessage = native
				}
				out = append(out, d)
			}
			diagnostics = out
		} else if native != "" {
			// AB_CONCRETE_OVERRIDE/NO_ANCESTOR: an invalid abstract
			// redeclaration does not introduce an overridable method for a leaf.
			d := declarationContractDiagnostic(typ, member.Range, native)
			d.NativeMessage = native
			diagnostics = append(diagnostics, d)
		}
	}
	return diagnostics
}

func abstractHierarchyVoidParent(parent typeMembers) bool {
	if !hasModifier(parent.modifiers, "abstract") || parent.nestingDepth != 0 {
		return false
	}
	for _, methods := range parent.methods {
		for _, method := range methods {
			if len(method.Parameters) == 0 && strings.EqualFold(method.Type, "void") &&
				hasModifier(method.Modifiers, "public") && hasModifier(method.Modifiers, "abstract") {
				return true
			}
		}
	}
	return false
}

// Select the nearest declaration before checking overridability. Skipping a
// concrete override and finding an older abstract declaration would incorrectly
// permit AB_CONCRETE_OVERRIDE's reabstracted method.
func abstractHierarchyNearestMethod(model *semaTypeMemberView, scope, parent string, member typesys.MemberSymbol) (typesys.MemberSymbol, typeMembers, bool) {
	seen := make(map[string]bool)
	for current := parent; current != "" && !seen[normalizeName(current)]; {
		seen[normalizeName(current)] = true
		owner, _, ok := semaLookupTypeMembers(model, current)
		if !ok || owner.dependency || owner.platform || owner.namespace != "" {
			break
		}
		for _, candidate := range owner.methods[normalizeName(member.Name)] {
			if sameInheritedSemaSignature(model, owner.name, candidate, scope, member) {
				return candidate, owner, true
			}
		}
		current = resolveNestedTypeName(model, owner.name, owner.superClass)
	}
	return typesys.MemberSymbol{}, typeMembers{}, false
}

func abstractHierarchyInvalidOverride(model *semaTypeMemberView, owner typeMembers, member typesys.MemberSymbol, seen map[string]bool) string {
	if !hasModifier(member.Modifiers, "abstract") || !hasModifier(member.Modifiers, "override") ||
		len(member.Parameters) != 0 || !strings.EqualFold(member.Type, "void") || owner.nestingDepth != 0 || seen[normalizeName(owner.name)] {
		return ""
	}
	seen[normalizeName(owner.name)] = true
	parent := resolveNestedTypeName(model, owner.name, owner.superClass)
	inherited, declaring, found := abstractHierarchyNearestMethod(model, owner.name, parent, member)
	if !found {
		return "@Override specified for non-overriding method: "
	}
	if !hasModifier(inherited.Modifiers, "virtual") && !hasModifier(inherited.Modifiers, "abstract") {
		return "Non-virtual, non-abstract methods cannot be overridden: "
	}
	return abstractHierarchyInvalidOverride(model, declaring, inherited, seen)
}
