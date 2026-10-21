package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaCacheReceiverScopeAPI(typ typesys.TypeSymbol) bool {
	if typ.Namespace != "" {
		return false
	}
	switch typ.EffectiveAPIVersion {
	case "62.0", "67.0", "68.0":
		return true
	}
	return false
}

// An own empty Request declaration wins over the platform class. Its missing
// static method cannot be accepted by a platform signature shortcut.
func semaCacheLexicalRequestCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiver, method, callee string, argumentCount, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if !semaCacheReceiverScopeAPI(typ) || !strings.EqualFold(receiver, "Request") ||
		!strings.EqualFold(method, "getCurrent") || argumentCount != 0 {
		return diagnostic.Diagnostic{}, false
	}
	name := resolveNestedTypeName(model, typ.Name, receiver)
	if !strings.EqualFold(name, typ.Name+"."+receiver) {
		return diagnostic.Diagnostic{}, false
	}
	target, ok := model.lookupName(name)
	if !ok || target.kind != apexast.DeclarationClass || target.platform || target.dependency || target.nestingDepth != 1 ||
		len(target.methods) != 0 || len(target.fields) != 0 || target.superClass != "" || len(target.interfaces) != 0 {
		return diagnostic.Diagnostic{}, false
	}
	return unknownCallDiagnostic(typ, member, callee, start, end, source), true
}

func semaCacheUnrelatedReceiverAlias(model *semaTypeMemberView, current typesys.TypeSymbol, receiver string, selected typeMembers) bool {
	requestAPI65 := current.Namespace == "" && current.EffectiveAPIVersion == "65.0" && strings.EqualFold(receiver, "Request")
	if (!semaCacheReceiverScopeAPI(current) && !requestAPI65) || strings.Contains(receiver, ".") || selected.platform ||
		selected.dependency || selected.sobject || selected.nestingDepth == 0 {
		return false
	}
	switch strings.ToLower(receiver) {
	case "request", "limits", "cache", "platformcache":
	default:
		return false
	}
	return !strings.EqualFold(resolveNestedTypeName(model, current.Name, receiver), selected.name)
}
