package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// The captured rows reject an unqualified, absent custom object's SObjectType
// receiver on both routes at API 62/67/68. The SObject-shaped spelling alone
// is insufficient: the token must name a type in this source/schema model.
// Dynamic getGlobalDescribe lookups (016/032) have no static token and retain
// their native false result. Qualified package tokens need separate evidence.
func (a *Analyzer) dependencyGateMissingSObjectTypeDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	name := ""
	switch expr.Kind {
	case ir.ExprVariable:
		name = expr.Name
	case ir.ExprCall:
		if expr.Left != nil && expr.Left.Kind == ir.ExprVariable {
			name = expr.Left.Name
		} else if receiver, _, qualified := splitSemaMethodPath(expr.Callee); qualified {
			name = receiver
		}
	}
	root, field, dotted := strings.Cut(name, ".")
	customSuffix := strings.HasSuffix(strings.ToLower(root), "__c")
	if !dotted || !strings.EqualFold(field, "SObjectType") || !customSuffix || strings.Count(root, "__") != 1 {
		return diagnostic.Diagnostic{}, false
	}
	if _, bound := scope.lookup(root); bound {
		return diagnostic.Diagnostic{}, false
	}
	resolved := resolveNestedTypeName(model, typ.Name, root)
	if _, known := model.lookup(normalizeName(resolved)); known || a.hasKnown(root) {
		return diagnostic.Diagnostic{}, false
	}
	at, end, found := semaIRNameSpan(source, start, name)
	if !found {
		at, end = start, start+len(name)
	}
	// Keep the compiler's own field-access wording; conformance compares the
	// rejection category, source unit and line rather than native prose.
	return semaFieldAccessDiagnostic(typ, member, name, "unknown SObject type "+root, at, end, source), true
}
