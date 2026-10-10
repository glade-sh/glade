package vm

import "testing"

func TestMapCloneShallowCopiesEntriesAndSharesSObjectValues(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Account acme = new Account(Name = 'Acme');
Map<Integer, Account> original = new Map<Integer, Account>{1 => acme};
Map<Integer, Account> cloned = original.clone();

cloned.put(2, new Account(Name = 'Beta'));
System.assertEquals(1, original.size());
System.assertEquals(2, cloned.size());
System.assert(!original.containsKey(2));

Account throughClone = cloned.get(1);
throughClone.Name = 'Updated';
System.assertEquals('Updated', original.get(1).Name);
System.assertEquals('Updated', cloned.get(1).Name);
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
