package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// The captured rows capture List<Account> indexed assignment and
// explicit set with an integer literal index and an inline Account query.
// Reuse scalar SOQL assignability; do not change add, other element types,
// query-result variables, aggregate queries or nonliteral index expressions.
func inlineQueryListSetDiagnostic(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiver, method string, args []ir.Expr, argTypes []string, pos int, source string, model *semaTypeMemberView, indexed bool) ([]diagnostic.Diagnostic, bool) {
	base, elements := semaGenericBaseAndArgs(receiver)
	if !strings.EqualFold(base, "List") || len(elements) != 1 || !strings.EqualFold(elements[0], "Account") ||
		semaProjectTypeShadowsPlatform(model, elements[0]) || !strings.EqualFold(method, "set") ||
		len(args) != 2 || len(argTypes) != 2 || args[0].Kind != ir.ExprLiteral || argTypes[0] != "Integer" || args[1].Kind != ir.ExprSOQL {
		return nil, false
	}
	queryBase, queryElements := semaGenericBaseAndArgs(argTypes[1])
	if !strings.EqualFold(queryBase, "List") || len(queryElements) != 1 {
		return nil, false
	}
	if strings.EqualFold(queryElements[0], "Account") && semaSOQLSingletonAssignable(elements[0], argTypes[1], "["+args[1].Value+"]", model) {
		return nil, true
	}
	// The captured row retains rejection; only its measured diagnostic changes.
	if indexed && strings.EqualFold(queryElements[0], "Contact") && !semaProjectTypeShadowsPlatform(model, queryElements[0]) {
		d := collectionCallDiagnostic(typ, member, method, len(args), pos, pos+len(method), source)
		d.NativeMessage = "Illegal assignment from List<Contact> to Account"
		return []diagnostic.Diagnostic{d}, true
	}
	return nil, false
}

// Only the captured direct indexed form gets the rejection projection. An
// explicit set call lowers identically but keeps its previous diagnostic.
func capturedIndexedQueryAssignment(expr ir.Expr, source string, pos int) bool {
	if expr.Kind != ir.ExprCall || expr.Callee != "set" || expr.Left == nil || expr.Left.Kind != ir.ExprVariable || len(expr.Args) != 2 || expr.Args[0].Kind != ir.ExprLiteral || pos < 0 || pos >= len(source) {
		return false
	}
	end := semaStatementEnd(source, pos)
	if end <= pos || end > len(source) {
		return false
	}
	statement := source[pos:end]
	eq := strings.IndexByte(statement, '=')
	if eq < 0 || eq+1 >= len(statement) || statement[eq+1] == '=' {
		return false
	}
	lhs := strings.Join(strings.Fields(statement[:eq]), "")
	return lhs == expr.Left.Name+"["+expr.Args[0].Value+"]"
}
