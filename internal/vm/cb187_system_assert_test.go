package vm

import "testing"

func TestCB187AssertExceptionQueryOnlyProcedures(t *testing.T) {
	program, err := CompileAnonymous(`
Exception assertion = new AssertException('failed assertion');
String expectedMessage = 'Procedure is only valid for System.QueryException';

Boolean fieldsCaught = false;
try {
	assertion.getInaccessibleFields();
} catch (Exception e) {
	fieldsCaught = true;
	System.assertEquals('System.TypeException', e.getTypeName());
	System.assertEquals(expectedMessage, e.getMessage());
}
System.assert(fieldsCaught, 'AssertException.getInaccessibleFields should throw');

Boolean causeCaught = false;
try {
	assertion.getCause();
} catch (Exception e) {
	causeCaught = true;
	System.assertEquals('System.TypeException', e.getTypeName());
	System.assertEquals(expectedMessage, e.getMessage());
}
System.assert(causeCaught, 'AssertException.getCause should throw');

Boolean initCaught = false;
try {
	assertion.initCause(new QueryException('cause'));
} catch (Exception e) {
	initCaught = true;
	System.assertEquals('System.TypeException', e.getTypeName());
	System.assertEquals(expectedMessage, e.getMessage());
}
System.assert(initCaught, 'AssertException.initCause should throw');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
