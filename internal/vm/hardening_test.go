package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
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

func TestNullIntegerArithmeticThrowsCatchableException(t *testing.T) {
	program, err := CompileAnonymous(`
Integer i = null;
Boolean caught = false;
try {
  Integer value = i + 1;
} catch (NullPointerException e) {
  caught = true;
}
System.assert(caught);
Integer value = 1;
caught = false;
try {
  value += i;
} catch (NullPointerException e) {
  caught = true;
}
System.assert(caught);
System.assertEquals(1, value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

// The compound-assignment check covers the shared local evaluator; direct
// binary arithmetic is covered by the API40 Salesforce proof regression.
func TestNullDecimalArithmeticThrowsCatchableException(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal d = null;
Boolean caught = false;
try {
  Decimal value = d + 2;
} catch (NullPointerException e) {
  caught = true;
}
System.assert(caught);
Decimal value = 2;
caught = false;
try {
  value *= d;
} catch (NullPointerException e) {
  caught = true;
}
System.assert(caught);
System.assertEquals(2, value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDecimalBinaryDoesNotRoundEachOperationToTwelvePlaces(t *testing.T) {
	got, err := evalBinary("*", Decimal(0.1234567890123), Int(1))
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != ValueDecimal || got.Decimal != 0.1234567890123 {
		t.Fatalf("decimal result = %#v, want unrounded 0.1234567890123", got)
	}
}

func TestPermissiveCPULimitViolationTracksLatestOverrun(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
total = total + 1;
total = total + 2;
total = total + 3;
total = total + 4;
System.assertEquals(10, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	caps := defaultLimitCaps()
	caps.CPUTimeMS = 1
	machine.SetLimitCaps(caps)
	result, err := machine.Execute(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.LimitViolations) != 1 || result.LimitViolations[0].Name != "cpuTime" {
		t.Fatalf("violations = %#v", result.LimitViolations)
	}
	if result.LimitViolations[0].Used <= caps.CPUTimeMS {
		t.Fatalf("cpu violation used = %d, want over limit %d", result.LimitViolations[0].Used, caps.CPUTimeMS)
	}
	if result.LimitViolations[0].Used != machine.cpuBudgetUsed {
		t.Fatalf("cpu violation used = %d, budget counter = %d", result.LimitViolations[0].Used, machine.cpuBudgetUsed)
	}
}

func TestSOQLForUpdateDoesNotPersistApprovalLock(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name = 'Acme');
Account locked = [SELECT Id FROM Account WHERE Name = 'Acme' LIMIT 1 FOR UPDATE];
System.assertEquals(false, Approval.isLocked(locked.Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	for _, record := range org.Objects["Account"].Records {
		if record.Fields["Name"].String == "Acme" && record.System.Locked {
			t.Fatalf("FOR UPDATE persisted lock on stored Account %s", record.ID)
		}
	}
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
