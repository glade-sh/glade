package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

func (vm *VM) coerceCast(typeName string, value Value) (Value, error) {
	return vm.coerceCastForOperand(typeName, value, false)
}

// Only the immediate operand counts. An Object bridge or a call on a query
// result must not inherit the query's singleton-assignment behavior.
func listSObjectCastDirectQueryOperand(expr ir.Expr) bool {
	if expr.Kind == ir.ExprSOQL {
		parts := strings.Fields(expr.Value)
		return len(parts) > 0 && strings.EqualFold(parts[0], "SELECT")
	}
	if expr.Kind != ir.ExprCall {
		return false
	}
	callee := expr.Callee
	if expr.Left != nil {
		if expr.Left.Kind != ir.ExprVariable {
			return false
		}
		callee = expr.Left.Name + "." + callee
	}
	for _, name := range []string{
		"Database.query", "Database.queryWithBinds",
		"Database.getQueryLocator", "Database.getQueryLocatorWithBinds",
	} {
		if strings.EqualFold(callee, name) {
			return true
		}
	}
	return false
}
