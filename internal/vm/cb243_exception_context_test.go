package vm

import "testing"

func TestConstructedExceptionCapturesAnonymousContext(t *testing.T) {
	program, err := CompileAnonymous(`List<Object> rows = new List<Object>();
Exception constructed = new AssertException('assert message');
System.assertEquals(2, constructed.getLineNumber());
System.assertEquals('AnonymousBlock: line 2, column 1', constructed.getStackTraceString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestConstructedExceptionUsesAnonymousColumnAndDefaultNullPointerMessage(t *testing.T) {
	program, err := CompileAnonymous(`NullPointerException nullPointer = new NullPointerException();
System.assertEquals('Script-thrown exception', nullPointer.getMessage());
System.assertEquals('System.NullPointerException: Script-thrown exception', nullPointer.toString());
System.assertEquals('AnonymousBlock: line 1, column 1', nullPointer.getStackTraceString());
ProcedureException procedure = new ProcedureException('procedure message');
System.assertEquals('AnonymousBlock: line 5, column 1', procedure.getStackTraceString());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
