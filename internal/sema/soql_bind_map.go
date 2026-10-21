package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// A dynamic query's bind map contains ordinary Apex values. An unresolved
// class qualifier in those values is a compile error, before query execution.
func (a *Analyzer) queryBindMapVariableDiagnostic(typ typesys.TypeSymbol, expr ir.Expr, scope irSemaScope, model *semaTypeMemberView, start int, source string) (diagnostic.Diagnostic, bool) {
	if !strings.EqualFold(expr.Callee, "Database.queryWithBinds") || len(expr.Args) < 2 || scope.hasNonFieldBinding("Database") || semaProjectTypeShadowsPlatform(model, "Database") {
		return diagnostic.Diagnostic{}, false
	}
	var missing func(ir.Expr) string
	missing = func(value ir.Expr) string {
		if value.Kind == ir.ExprVariable && strings.Contains(value.Name, ".") && !strings.HasSuffix(strings.ToLower(value.Name), ".class") && !a.irVariableKnown(value.Name, scope, model, typ.Name) {
			root, _, _ := strings.Cut(value.Name, ".")
			return root
		}
		if value.Left != nil {
			if name := missing(*value.Left); name != "" {
				return name
			}
		}
		if value.Right != nil {
			if name := missing(*value.Right); name != "" {
				return name
			}
		}
		for _, arg := range value.Args {
			if name := missing(arg); name != "" {
				return name
			}
		}
		for _, arg := range value.NamedArgs {
			if name := missing(arg.Expr); name != "" {
				return name
			}
		}
		return ""
	}
	name := missing(expr.Args[1])
	if name == "" {
		return diagnostic.Diagnostic{}, false
	}
	if at, _, found := semaIRNameSpan(source, start, name); found {
		start = at
	}
	message := "Variable does not exist: " + name
	return diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, start, start+len(name))}, true
}
