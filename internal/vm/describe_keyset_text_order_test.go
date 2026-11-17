package vm

import (
	"strings"
	"testing"
)

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

// NATIVE review_round8 ks_add_custom, ks_add_account and ks_set_ops, 62.0 and
// 67.0: the field map's key set refuses a write with an uncatchable
// FinalException; a Set built from it is an ordinary writable Set.
func TestDescribeKeySetIsReadOnlyAndItsCopyIsNot(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
Set<String> keys = Schema.SObjectType.Account.fields.getMap().keySet();
Set<String> copy = new Set<String>(keys);
copy.add('aaa_added'); copy.add('zzz_added');
System.assert(copy.contains('aaa_added'));
System.assertEquals(keys.size() + 2, copy.size());
try { keys.add('x'); System.assert(false, 'add must fail'); } catch (Exception e) { System.assert(false, 'caught ' + e.getTypeName()); }
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Collection is read-only") {
		t.Fatalf("add on the key set: %v", err)
	}
}
