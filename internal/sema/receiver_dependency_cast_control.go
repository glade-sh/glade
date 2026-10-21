package sema

import (
	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// Installing source dependencies cannot make a direct String-literal to Integer cast
// uncertain. Object bridges and other primitive/operand shapes are unchanged.
func receiverDependencyLiteralCastControl(typ typesys.TypeSymbol, operand ir.Expr, model *semaTypeMemberView) bool {
	if typ.Dependency || typ.Namespace != "" || operand.Kind != ir.ExprLiteral ||
		model == nil || model.state == nil || model.state.base == nil || !model.state.base.receiverDependencyInstalled {
		return false
	}
	major, known := apexversion.Major(typ.EffectiveAPIVersion)
	return known && major >= 59 && major <= 68
}
