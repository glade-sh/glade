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
