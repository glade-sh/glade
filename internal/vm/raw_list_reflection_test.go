package vm

import "testing"

func TestTypedListReflectionContractsRemainUnchanged(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Type stringsType=Type.forName('List<String>');
System.assertNotEquals(null,stringsType);
System.assert(stringsType.isAssignableFrom(stringsType));
System.assertEquals(false,stringsType.isAssignableFrom(Type.forName('Set<String>')));
`, CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
