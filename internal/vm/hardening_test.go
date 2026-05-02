package vm

import (
	"testing"

	"github.com/open-aer/oaer/internal/ir"
)

func TestNoPanicOnMalformedOrUnsupportedProgram(t *testing.T) {
	assertNoPanic(t, func() {
		_, _ = CompileAnonymous("public class NotAnonymous {")
	})
	assertNoPanic(t, func() {
		_, _ = Execute(ir.Program{Instructions: []ir.Instruction{{Op: "unsupported"}}}, nil)
	})
	assertNoPanic(t, func() {
		_, _ = Execute(ir.Program{Instructions: []ir.Instruction{{
			Op: ir.OpExpr,
			Expr: ir.Expr{
				Kind:     ir.ExprBinary,
				Operator: "???",
				Left:     &ir.Expr{Kind: ir.ExprLiteral, Value: "1"},
				Right:    &ir.Expr{Kind: ir.ExprLiteral, Value: "2"},
			},
		}}}, nil)
	})
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
