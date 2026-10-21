package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// S09 rejects an expression statement immediately after constructing and
// throwing an ordinary nested exception in a try body. Other post-throw
// shapes retain their existing contracts until independently captured.
func (a *Analyzer) capturedTryThrowSuccessor(typ typesys.TypeSymbol, member typesys.MemberSymbol, items []ir.Instruction, scope irSemaScope, offset int, source string, model *semaTypeMemberView) []diagnostic.Diagnostic {
	for i := 1; i < len(items); i++ {
		prior, next := items[i-1], items[i]
		if prior.Op != ir.OpThrow || prior.Expr.Kind != ir.ExprCall || !strings.HasPrefix(prior.Expr.Callee, "new:") || next.Op != ir.OpExpr || next.Expr.Kind != ir.ExprCall {
			continue
		}
		thrown := a.inferIRExprType(prior.Expr, scope, model, typ.Name)
		owner, found := model.lookupName(thrown)
		if !found || owner.platform || owner.dependency || owner.nestingDepth == 0 || !semaAssignableToType("Exception", thrown, model) {
			continue
		}
		d := statementContractDiagnostic(typ, member, "instruction is unreachable", offset+next.Pos, source)
		d.NativeMessage = "Unreachable statement"
		return []diagnostic.Diagnostic{d}
	}
	return nil
}
