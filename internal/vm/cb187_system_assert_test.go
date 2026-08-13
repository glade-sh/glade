package vm

import "testing"

func TestCB187AssertExceptionRejectsOnlyGetInaccessibleFields(t *testing.T) {
	program, err := CompileAnonymous(`
Exception assertion = new AssertException('failed assertion');
Boolean caught = false;
try {
	assertion.getInaccessibleFields();
} catch (Exception e) {
	caught = true;
	System.assertEquals('System.TypeException', e.getTypeName());
	System.assertEquals('Procedure is only valid for System.QueryException', e.getMessage());
}
System.assert(caught, 'AssertException.getInaccessibleFields should throw');

Exception query = new QueryException('failed query');
System.assertNotEquals(null, query.getInaccessibleFields());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
