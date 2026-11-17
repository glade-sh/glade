package vm

import "testing"

// The field map key set iterates in platform order and renders its text
// alphabetically; the order flag is a field of Value, so it travels with copies
// of the set and never reaches a set built from it.
func TestDescribeKeySetTextOrderFlag(t *testing.T) {
	program, err := CompileAnonymous(`
Set<String> keys = Schema.SObjectType.Account.fields.getMap().keySet();
List<String> platform = new List<String>(keys);
List<String> sorted = new List<String>(platform);
sorted.sort();
System.assertNotEquals(sorted, platform, 'platform order is not alphabetical');
String expected = '{' + String.join(new List<String>{sorted[0], sorted[1], sorted[2]}, ', ');
System.assert(('' + keys).startsWith(expected), 'text: ' + keys);
Set<String> copy = keys;
Object asObject = keys;
Set<String> casted = (Set<String>) asObject;
System.assert(('' + copy).startsWith(expected), 'assigned copy: ' + copy);
System.assert(('' + casted).startsWith(expected), 'cast copy: ' + casted);
System.assertEquals(platform, new List<String>(casted), 'cast iteration keeps the platform order');
Set<String> built = new Set<String>(platform);
System.assert(('' + built).startsWith('{' + platform[0] + ', ' + platform[1]), 'a set built from the list keeps its own text order');
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
}
