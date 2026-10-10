package vm

import "testing"

// Source contract: apex.behavior.language.null-coalescing.empty-query-fallback
// at API 67.
func TestExecNullCoalescingInlineSOQLEmptyFallbackAPI67(t *testing.T) {
	run := func(t *testing.T, source string) {
		t.Helper()
		program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "67.0"})
		if err != nil {
			t.Fatal(err)
		}
		if program.APIVersion != "67.0" {
			t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
		}
		machine := New(nil)
		org := testDataOrg()
		machine.SetOrg(&org)
		if _, err := machine.Execute(program); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("empty-inline-query-uses-fallback", func(t *testing.T) {
		run(t, `
Account selected = [SELECT Id, Name FROM Account WHERE Name = 'no matching account' LIMIT 1]
    ?? new Account(Name = 'fallback');
System.assertEquals('fallback', selected.Name);
`)
	})

	t.Run("empty-collection-is-not-null", func(t *testing.T) {
		run(t, `
List<Account> empty = new List<Account>();
List<Account> selected = empty ?? new List<Account>{new Account(Name = 'fallback')};
System.assertEquals(0, selected.size());
`)
	})

	t.Run("one-row-query-keeps-value-and-skips-lazy-rhs", func(t *testing.T) {
		run(t, `
Account seed = new Account(Name = 'winner');
insert seed;
Account fallbackMarker;
Account selected = [SELECT Id, Name FROM Account WHERE Id = :seed.Id LIMIT 1]
    ?? (fallbackMarker = new Account(Name = 'fallback'));
System.assertEquals(seed.Id, selected.Id);
System.assertEquals('winner', selected.Name);
System.assertEquals(null, fallbackMarker);
`)
	})
}
