package vm

import "testing"

func TestExecTypeForNameApi67NamespaceContract(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(null, Type.forName('System', 'String'));
System.assertEquals(null, Type.forName('System', 'Exception'));
System.assertEquals(null, Type.forName('System.String'));
System.assertNotEquals(null, Type.forName('Schema', 'DisplayType'));
System.assertNotEquals(null, Type.forName('ApexPages', 'StandardController'));
System.assertNotEquals(null, Type.forName('', 'String'));
System.assertNotEquals(null, Type.forName(null, 'String'));
System.assertEquals(null, Type.forName('DefinitelyInvalidNamespace', 'String'));

System.assertNotEquals(null, Type.forName('String'));
System.assertNotEquals(null, Type.forName('Account'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
