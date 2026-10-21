package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/ir"
	"strings"
)

// CompileSOQLBindExpression uses the existing Apex expression grammar without
// inventing another parser for an inline SOQL operand.
func CompileSOQLBindExpression(source string) (ir.Expr, error) {
	expr, end, err := compileExpressionPrefix(source)
	if err != nil {
		return ir.Expr{}, err
	}
	if strings.TrimSpace(source[end:]) != "" {
		return ir.Expr{}, fmt.Errorf("unexpected SOQL bind expression suffix at byte %d", end)
	}
	return expr, nil
}
