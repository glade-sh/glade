package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// The native capture rejects a local class extending the same-named, explicitly
// qualified nonvirtual package class. Other dependency inheritance remains
// on its existing uncertainty path; R31's abstract package base stays valid.
func receiverDependencyInheritanceDiagnostic(typ typesys.TypeSymbol, parent typeMembers) (diagnostic.Diagnostic, bool) {
	if !receiverDependencyRejectionAPI(typ) || !typ.HasSourceSnapshot() ||
		!parent.dependency || !parent.receiverDependencySource || parent.namespace == "" ||
		parent.kind != apexast.DeclarationClass || parent.nestingDepth != 0 ||
		!strings.EqualFold(typ.SuperClass, parent.name) ||
		!strings.EqualFold(parent.name, parent.namespace+"."+typ.Name) ||
		hasModifier(parent.modifiers, "virtual") || hasModifier(parent.modifiers, "abstract") {
		return diagnostic.Diagnostic{}, false
	}
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "receiver_expression_inheritance", Message: fmt.Sprintf("class %q cannot extend non-virtual, non-abstract class %q", typ.Name, typ.SuperClass), File: typ.File, Range: &typ.Range}, true
}
