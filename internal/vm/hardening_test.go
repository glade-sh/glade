package vm

import (
	"fmt"
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
	assertNoPanic(t, func() {
		_, _ = Execute(ir.Program{Instructions: []ir.Instruction{{
			Op:   ir.OpExpr,
			Expr: ir.Expr{Kind: ir.ExprBinary, Operator: "+"},
		}}}, nil)
	})
}

func TestNullDereferenceThrowsCatchableException(t *testing.T) {
	program, err := CompileAnonymous(`
Account a = null;
String message = '';
try {
	a.get('Name');
} catch (NullPointerException e) {
	message = e.getMessage();
}
System.assertEquals('Attempt to de-reference a null object', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	assertNoPanic(t, func() {
		if _, err := Execute(program, nil); err != nil {
			t.Fatal(err)
		}
	})
}

func TestExecuteRecoversInternalPanics(t *testing.T) {
	program, err := CompileAnonymous("System.debug('boom');")
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(panicWriter{}).Execute(program)
	if err == nil || err.Error() != "internal VM panic: writer exploded" {
		t.Fatalf("err = %v", err)
	}
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) {
	panic(fmt.Errorf("writer exploded"))
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
