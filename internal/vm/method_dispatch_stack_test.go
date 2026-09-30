package vm

import "testing"

func TestCallMethodStackUsesCallerCallSite(t *testing.T) {
	calleeProgram, err := CompileAnonymous(`
DmlException failure = new DmlException('failed');
throw failure;
`)
	if err != nil {
		t.Fatal(err)
	}
	callerProgram, err := CompileAnonymous(`
try {
  Callee.fail();
} catch (DmlException failure) {
  System.assert(String.valueOf(failure.getStackTraceString()).contains('Class.Caller.run: line 3'), failure.getStackTraceString());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Caller"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Callee"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "Callee.fail",
		ClassName: "Callee",
		IsStatic:  true,
		Program:   calleeProgram,
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{
		Name:      "Caller.run",
		ClassName: "Caller",
		IsStatic:  true,
		Program:   callerProgram,
	}); err != nil {
		t.Fatal(err)
	}
	entry, err := CompileAnonymous("Caller.run();")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(entry); err != nil {
		t.Fatal(err)
	}
}
