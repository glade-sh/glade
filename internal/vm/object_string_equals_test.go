package vm

import "testing"

func TestObjectStringEqualsPreservesOtherViews(t *testing.T) {
	program, err := CompileAnonymous(`
Object upper='NORTH'; Object lower='north';
System.assertEquals(true,upper.equals(lower));
System.assertEquals(true,upper==lower);
System.assertEquals(false,((String)upper).equals((String)lower));
String typed='NORTH';
System.assertEquals(false,typed.equals('north'));
System.assertEquals(false,upper.equals('south'));
// The following are preservation controls, not newly captured native claims.
System.assertEquals(false,upper.equals(null));
System.assertEquals(false,upper.equals(5));
Object numericValue=5; System.assertEquals(true,numericValue.equals(5));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.VarTypes["text"] = "Object"
	for _, tc := range []struct{ receiver, argument Value }{
		{Value{Kind: ValueString, Type: "Id", Text: "001000000000001"}, String("001000000000001")},
		{String("001000000000001"), Value{Kind: ValueString, Type: "Id", Text: "001000000000001"}},
	} {
		if _, handled := machine.objectViewStringEquals("text", tc.receiver, "equals", []Value{tc.argument}); handled {
			t.Fatal("Object-held Id pair entered the String-only path")
		}
	}
}
