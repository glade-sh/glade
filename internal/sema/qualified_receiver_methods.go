package sema

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// semaQualifiedReceiverCallDiagnostic checks method calls on the qualified
// receivers captured in qualified_receivers/compile.tsv (API 62.0 and 67.0,
// anonymous and isTest):
//
//   - Label.X.m() and System.Label.X.m() on a label missing from a known
//     catalogue: "External string does not exist: X" (missing_label,
//     missing_system_label), the same diagnostic as the bare read.
//   - Schema.<SObject>.m() for a method no SObject signature declares:
//     "... void m() from the type <SObject>" (invalid_method).
//   - <SObject>.SObjectType.m() for a method Schema.SObjectType does not
//     declare: "... void m() from the type Schema.SObjectType"
//     (invalid_token_method).
func (a *Analyzer) semaQualifiedReceiverCallDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, receiverExpr, method string, scope irSemaScope, pos, bodyOffset int, source string, model *semaTypeMemberView) (diagnostic.Diagnostic, bool) {
	if expr.Left != nil {
		return diagnostic.Diagnostic{}, false
	}
	if item, rejected := semaCustomLabelDiagnostic(typ, receiverExpr, scope, model, bodyOffset+pos, source); rejected {
		return item, true
	}
	parts := strings.Split(receiverExpr, ".")
	if _, scoped := scope.lookup(parts[0]); scoped || semaProjectTypeShadowsPlatform(model, parts[0]) {
		return diagnostic.Diagnostic{}, false
	}
	schemaQualified := strings.EqualFold(parts[0], "Schema")
	if schemaQualified {
		parts = parts[1:]
	}
	if len(parts) == 0 || semaQualifierNamesKnownNamespace(a, parts[0]) || semaProjectTypeShadowsPlatform(model, parts[0]) ||
		!semaSObjectExistsInModel(parts[0], model) || !semaSObjectIdentityEstablished(a, parts[0], model) {
		return diagnostic.Diagnostic{}, false
	}
	var owner string
	switch {
	case schemaQualified && len(parts) == 1:
		object := parts[0]
		if members, ok := model.lookup(normalizeName(object)); ok && members.name != "" {
			object = members.name
		}
		if _, ok := semaPlatformMethodSignatureForMode(model, object, method, "class"); ok {
			return diagnostic.Diagnostic{}, false
		}
		if _, ok := semaPlatformMethodSignatureForMode(model, object, method, "instance"); ok {
			return diagnostic.Diagnostic{}, false
		}
		if _, ok := semaPlatformMethodSignatureForMode(model, "SObject", method, "instance"); ok {
			return diagnostic.Diagnostic{}, false
		}
		if len(resolveMemberMethods(model, object, method)) != 0 {
			return diagnostic.Diagnostic{}, false
		}
		owner = object
	case len(parts) == 2 && strings.EqualFold(parts[1], "SObjectType"):
		if _, ok := semaPlatformMethodSignatureForMode(model, "Schema.SObjectType", method, "instance"); ok {
			return diagnostic.Diagnostic{}, false
		}
		if len(resolveMemberMethods(model, "Schema.SObjectType", method)) != 0 {
			return diagnostic.Diagnostic{}, false
		}
		owner = "Schema.SObjectType"
	default:
		return diagnostic.Diagnostic{}, false
	}
	message := fmt.Sprintf("Method does not exist or incorrect signature: void %s(%s) from the type %s", method, strings.Join(irCallArgTypes(a, expr.Args, scope, model, typ.Name), ","), owner)
	d := unknownCallDiagnostic(typ, member, expr.Callee, bodyOffset+pos, bodyOffset+pos+max(1, len(expr.Callee)), source)
	d.NativeMessage = message
	return d, true
}
