package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// CD136 rows observe named cross-namespace source at API62 only. Keep
// other APIs, artifacts, anonymous callers and other dependency shapes on
// their existing paths until their independent native rows are available.
func namespaceInterfaceNativeCaller(typ typesys.TypeSymbol) bool {
	api, known := apexversion.Major(typ.EffectiveAPIVersion)
	return known && api == 62 && !typ.Dependency && !typ.Artifact && typ.HasSourceSnapshot() && typ.Namespace != "" && typ.Kind == apexast.DeclarationClass && typ.NestingDepth == 0
}

func namespaceInterfaceNativeOwner(typ typesys.TypeSymbol, owner typeMembers) bool {
	api, known := apexversion.Major(owner.effectiveAPIVersion)
	return namespaceInterfaceNativeCaller(typ) && known && api == 62 && owner.kind == apexast.DeclarationInterface && owner.dependency && owner.receiverDependencySource && !owner.platform && owner.nestingDepth == 0 && owner.namespace != "" && !strings.EqualFold(owner.namespace, typ.Namespace)
}

func namespaceInterfaceVisibilityDiagnostic(typ typesys.TypeSymbol, spelling string, owner typeMembers) (diagnostic.Diagnostic, bool) {
	if !namespaceInterfaceNativeOwner(typ, owner) || accessModifier(typ.Modifiers) != "public" || accessModifier(owner.modifiers) != "public" || !strings.EqualFold(spelling, owner.name) || !strings.HasPrefix(strings.ToLower(spelling), strings.ToLower(owner.namespace)+".") {
		return diagnostic.Diagnostic{}, false
	}
	message := "Type is not visible: " + owner.name
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "receiver_expression_interface_access", Message: message, NativeMessage: message, File: typ.File, Range: &typ.Range}, true
}

func namespaceInterfaceOverloadDiagnostic(d diagnostic.Diagnostic, typ typesys.TypeSymbol, receiver, method string, args []string, candidates []resolvedMember, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if d.Code != "GLADESEMA022" || !typ.IsTest || !namespaceInterfaceNativeCaller(typ) || len(args) != 1 || !strings.EqualFold(args[0], "null") || len(candidates) != 2 {
		return d, false
	}
	text, record := false, false
	for _, candidate := range candidates {
		owner, _, ok := semaLookupTypeMembers(model, candidate.owner)
		if !ok || !namespaceInterfaceNativeOwner(typ, owner) || accessModifier(owner.modifiers) != "global" || !strings.EqualFold(receiver, owner.name) || len(candidate.member.Parameters) != 1 || candidate.member.Kind != apexast.DeclarationMethod || !strings.EqualFold(candidate.member.Type, "String") || hasModifier(candidate.member.Modifiers, "static") {
			return d, false
		}
		parameter := candidate.member.Parameters[0].Type
		if strings.EqualFold(parameter, "String") {
			text = true
		} else if object, _, found := semaLookupTypeMembers(model, parameter); found && object.sobject && !object.syntheticStandardSObject && strings.HasPrefix(strings.ToLower(parameter), strings.ToLower(owner.namespace)+"__") {
			record = true
		} else {
			return d, false
		}
	}
	if !text || !record {
		return d, false
	}
	// Retain this definite observed error without hardening the generic
	// source-dependency warning policy for unrelated or unresolved calls.
	d.Code = "receiver_expression_interface_overload"
	d.NativeMessage = "Ambiguous method signature: void " + method + "(NULL)"
	return d, true
}
