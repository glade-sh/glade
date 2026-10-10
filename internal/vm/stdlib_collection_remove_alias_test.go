package vm

import "testing"

func TestExecSetRemovePreservesAliasesAndCloneAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Set<Integer> original = new Set<Integer>{1, 2, 3};
Set<Integer> alias = original;
Set<Integer> independent = original.clone();

Boolean removed = original.remove(3);
System.assertEquals(true, removed);
System.assertEquals(2, original.size());
System.assertEquals(2, alias.size());
System.assertEquals(false, alias.contains(3));
System.assertEquals(3, independent.size());
System.assert(independent.contains(3));

Boolean removedMissing = original.remove(99);
System.assertEquals(false, removedMissing);
System.assertEquals(2, original.size());
System.assertEquals(2, alias.size());
System.assertEquals(3, independent.size());

Boolean removedThroughAlias = alias.remove(2);
System.assertEquals(true, removedThroughAlias);
System.assertEquals(1, original.size());
System.assertEquals(false, original.contains(2));
System.assertEquals(1, alias.size());
System.assertEquals(false, alias.contains(2));
System.assertEquals(3, independent.size());
System.assert(independent.contains(2));
System.assert(independent.contains(3));
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
