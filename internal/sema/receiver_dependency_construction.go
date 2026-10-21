package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// The native capture is the named-route no-argument construction rejection twin of
// package-lexical R37. The package's public type stays usable inside the
// package; its global constructor/type neighbors R04/R39 remain accessible.
func receiverDependencyConstructionDiagnostic(typ typesys.TypeSymbol, spelling, resolved string, argc int, start, end int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if !receiverDependencyRejectionAPI(typ) || !typ.IsTest || argc != 0 || !strings.EqualFold(spelling, resolved) {
		return diagnostic.Diagnostic{}, false
	}
	target, ok := model.lookupName(resolved)
	if !ok || !target.dependency || !target.receiverDependencySource || target.namespace == "" ||
		target.nestingDepth != 0 || target.kind != apexast.DeclarationClass ||
		accessModifier(target.modifiers) != "public" || !strings.HasPrefix(strings.ToLower(spelling), strings.ToLower(target.namespace)+".") {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "receiver_expression_type_access", Message: fmt.Sprintf("Type is not visible: %s", target.name), File: typ.File, Range: semaRange(source, start, end)}, true
}
