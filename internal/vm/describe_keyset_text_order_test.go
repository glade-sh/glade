package vm

import (
	"strings"
	"testing"
)

// The field map key set iterates in platform order and renders its text
// alphabetically; the order flag is a field of Value, so it travels with copies
// of the set and to sets built from it.
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
System.assert(('' + built).startsWith(expected), 'review_round9 ks_list_set: a Set built from the key set names renders alphabetically: ' + built);
System.assertEquals(platform[0], new List<String>(built)[0], 'iteration keeps the list order');
Set<String> filled = new Set<String>(); filled.addAll(keys);
System.assert(('' + filled).startsWith(expected), 'review_round9 ks_u_addall: ' + filled);
Set<String> cloned = keys.clone(); cloned.add('aaa_added'); cloned.addAll(new Set<String>{'zzz_added'});
System.assert(cloned.contains('aaa_added') && cloned.contains('zzz_added'), 'review_round9 ks_clone_add: a clone is writable');
Set<String> plainFirst = new Set<String>{'zeta', 'alpha'};
System.assert(('' + plainFirst).startsWith('{zeta'), 'an ordinary Set keeps its own order');
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

// NATIVE review_round9 ks_plain_map_add, 62.0 and 67.0: a plain Map's key set
// refuses add with the same FinalException, and a clone of it is writable.
func TestPlainMapKeySetIsReadOnly(t *testing.T) {
	machine := New(nil)
	program, err := CompileAnonymous(`
Map<String,Integer> m = new Map<String,Integer>{'a' => 1};
Set<String> copy = m.keySet().clone(); copy.add('b');
System.assertEquals(2, copy.size());
System.assertEquals(1, m.size());
m.keySet().add('c');
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	if err == nil || !strings.Contains(err.Error(), "Collection is read-only") {
		t.Fatalf("add on a Map key set: %v", err)
	}
}
