package vm

import "testing"

func TestExecStringFormatNullArgumentsMatchSalesforce(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
    String.format(null, null);
    System.assert(false, 'String.format should reject null arguments');
} catch (System.NullPointerException error) {
    caught = true;
    System.assertEquals('Argument cannot be null.', error.getMessage());
}
System.assert(caught, 'String.format null arguments must be catchable as NullPointerException');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestStringStaticFormatNullArgumentsMatchSalesforce(t *testing.T) {
	_, err := stringStatic("String.format", []Value{Null, Null})
	if err == nil {
		t.Fatal("String.format should reject null arguments")
	}
	if err.Error() != "Argument cannot be null." {
		t.Fatalf("String.format null error = %v", err)
	}
}
