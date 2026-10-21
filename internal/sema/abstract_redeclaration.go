package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// The earlier native redeclarations cover public, zero-argument String methods
// in an abstract subclass of an abstract source class. The hierarchy helper
// adds the captured void and inner Object shapes; keep other modifier checks.
func capturedAbstractRedeclarationDiagnostics(index typesys.Index, typ typesys.TypeSymbol, diagnostics []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	diagnostics = capturedAbstractHierarchyDiagnostics(index, typ, diagnostics)
	if typ.Dependency || typ.OwnerName != "" || typ.Namespace != "" || typ.SuperClass == "" || !hasModifier(typ.Modifiers, "abstract") || !typeUsesAPIVersionAtLeast(typ, 62) {
		return diagnostics
	}
	var model *semaTypeMemberView
	for _, member := range typ.Members {
		if member.Kind != apexast.DeclarationMethod || len(member.Parameters) != 0 ||
			!hasModifier(member.Modifiers, "public") || !hasModifier(member.Modifiers, "abstract") ||
			!hasModifier(member.Modifiers, "override") || hasModifier(member.Modifiers, "static") ||
			(!strings.EqualFold(member.Type, "String") && !strings.EqualFold(member.Type, "Integer")) {
			continue
		}
		if model == nil {
			model = buildSemaTypeMemberView(index)
		}
		parentName := resolveNestedTypeName(model, typ.Name, typ.SuperClass)
		parent, _, ok := semaLookupTypeMembers(model, parentName)
		if !ok || parent.dependency || !hasModifier(parent.modifiers, "abstract") {
			continue
		}
		inherited, _, found, objectFallback := overridableInheritedMethod(model, typ, member)
		allow := found && !objectFallback && strings.EqualFold(inherited.Type, "String") &&
			hasModifier(inherited.Modifiers, "public") && strings.EqualFold(member.Type, "String")
		native := ""
		if found && !objectFallback && hasModifier(inherited.Modifiers, "public") &&
			strings.EqualFold(inherited.Type, "String") && strings.EqualFold(member.Type, "Integer") {
			native = "Method return types clash: Integer vs String from the type " + typ.Name
		}
		if !found && strings.EqualFold(member.Type, "String") {
			// The captured negative twin names another method instead of the
			// parent's public abstract zero-argument String declaration.
			for _, methods := range parent.methods {
				for _, candidate := range methods {
					if len(candidate.Parameters) == 0 && strings.EqualFold(candidate.Type, "String") &&
						hasModifier(candidate.Modifiers, "public") && hasModifier(candidate.Modifiers, "abstract") {
						native = nativeLifecycleOverrideMessage(model, typ, member)
					}
				}
			}
		}
		if !allow && native == "" {
			continue
		}
		out := diagnostics[:0]
		for _, d := range diagnostics {
			if d.Message == fmt.Sprintf("method %q cannot be both abstract and override", member.Name) &&
				d.Range != nil && d.Range.Start.Offset == member.Range.Start.Offset {
				if allow {
					continue
				}
				d.NativeMessage = native
			}
			out = append(out, d)
		}
		diagnostics = out
	}
	return diagnostics
}
