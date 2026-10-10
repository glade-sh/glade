package vm

import "testing"

// Nine selected cases from the retained collection documentation catalog.
// Retained Summer '26/API67 catalog SHA-256:
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b.
// Each case exercises a bounded public clause, not full-method/API-interval parity.
// Primitive mutation visibility through aliases applies the reference-sharing
// inference supported by native collection controls. No corpus/DML is needed.
// Existing List.equals, Set.remove, clear and bulk-alias tests are selected as
// separate controls; this file does not duplicate their implementation.
func TestExecListSetMutationFamilyAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			// List /documents/2205/members/3,4,12,20.
			name: "listAppendAndIndexedInsertPreserveAlias",
			source: `
List<Integer> original = new List<Integer>{1, 3};
List<Integer> alias = original;
List<Integer> independent = original.clone();
original.add(4);
alias.add(1, 2);
System.assertEquals(4, original.size());
System.assertEquals(4, alias.size());
System.assertEquals(1, original.get(0));
System.assertEquals(2, original.get(1));
System.assertEquals(3, original.get(2));
System.assertEquals(4, original.get(3));
System.assertEquals(2, independent.size());
System.assertEquals(3, independent.get(1));
`,
		},
		{
			// List /documents/2205/members/5; source list order is preserved.
			name: "listAddAllListPreservesAliasAndSource",
			source: `
List<Integer> original = new List<Integer>{1};
List<Integer> alias = original;
List<Integer> additions = new List<Integer>{2, 2, 3};
original.addAll(additions);
System.assertEquals(4, alias.size());
System.assertEquals(2, alias.get(1));
System.assertEquals(2, alias.get(2));
System.assertEquals(3, alias.get(3));
alias.addAll(new List<Integer>());
System.assertEquals(4, original.size());
additions.set(0, 99);
System.assertEquals(2, original.get(1));
`,
		},
		{
			// List /documents/2205/members/6; no Set iteration order asserted.
			name: "listAddAllSetPreservesAliasAndSource",
			source: `
List<Integer> original = new List<Integer>{1};
List<Integer> alias = original;
Set<Integer> additions = new Set<Integer>{2, 3};
alias.addAll(additions);
System.assertEquals(3, original.size());
System.assert(original.contains(1));
System.assert(original.contains(2));
System.assert(original.contains(3));
original.addAll(new Set<Integer>());
System.assertEquals(3, alias.size());
additions.add(4);
System.assertEquals(false, original.contains(4));
`,
		},
		{
			// List /documents/2205/members/18,19; returned removed value and shifts.
			name: "listSetAndRemovePreserveAliasAndClone",
			source: `
List<Integer> original = new List<Integer>{1, 2, 3};
List<Integer> alias = original;
List<Integer> independent = original.clone();
alias.set(1, 9);
System.assertEquals(9, original.get(1));
Integer removed = original.remove(1);
System.assertEquals(9, removed);
System.assertEquals(2, alias.size());
System.assertEquals(1, alias.get(0));
System.assertEquals(3, alias.get(1));
System.assertEquals(3, independent.size());
System.assertEquals(2, independent.get(1));
`,
		},
		{
			// List /documents/2205/members/1,8,21; Integer ascending sort.
			name: "listSortPreservesAliasAndIndependentCopies",
			source: `
List<Integer> original = new List<Integer>{3, 1, 2, 1};
List<Integer> alias = original;
List<Integer> copied = new List<Integer>(original);
List<Integer> cloned = original.clone();
alias.sort();
System.assertEquals(1, original.get(0));
System.assertEquals(1, original.get(1));
System.assertEquals(2, original.get(2));
System.assertEquals(3, original.get(3));
System.assertEquals(3, copied.get(0));
System.assertEquals(3, cloned.get(0));
copied.set(0, 9);
cloned.add(4);
System.assertEquals(1, original.get(0));
System.assertEquals(4, original.size());
`,
		},
		{
			// Set /documents/2215/members/3,8,19; changed and duplicate Booleans.
			name: "setAddPreservesAliasAndDuplicateResult",
			source: `
Set<Integer> original = new Set<Integer>{1};
Set<Integer> alias = original;
Set<Integer> independent = original.clone();
System.assertEquals(true, original.add(2));
System.assertEquals(2, alias.size());
System.assert(alias.contains(2));
System.assertEquals(false, alias.add(2));
System.assertEquals(2, original.size());
System.assertEquals(true, alias.add(3));
System.assert(original.contains(3));
System.assertEquals(1, independent.size());
System.assertEquals(false, independent.contains(2));
`,
		},
		{
			// Set /documents/2215/members/4; List overload and changed/no-op result.
			name: "setAddAllListPreservesAliasAndChangedResult",
			source: `
Set<Integer> original = new Set<Integer>{1};
Set<Integer> alias = original;
Set<Integer> independent = original.clone();
List<Integer> additions = new List<Integer>{1, 2, 2, 3};
System.assertEquals(true, alias.addAll(additions));
System.assertEquals(3, original.size());
System.assert(original.contains(2));
System.assert(original.contains(3));
System.assertEquals(false, original.addAll(new List<Integer>{1, 3}));
System.assertEquals(false, alias.addAll(new List<Integer>()));
System.assertEquals(3, alias.size());
System.assertEquals(4, additions.size());
System.assertEquals(1, independent.size());
`,
		},
		{
			// Set /documents/2215/members/5; Set overload, changed/no-op and isolation.
			name: "setAddAllSetPreservesAliasAndChangedResult",
			source: `
Set<Integer> original = new Set<Integer>{1};
Set<Integer> alias = original;
Set<Integer> additions = new Set<Integer>{1, 2, 3};
System.assertEquals(true, original.addAll(additions));
System.assertEquals(3, alias.size());
System.assert(alias.contains(2));
System.assert(alias.contains(3));
System.assertEquals(false, alias.addAll(new Set<Integer>{2, 3}));
System.assertEquals(false, original.addAll(new Set<Integer>()));
additions.add(4);
System.assertEquals(false, original.contains(4));
System.assertEquals(3, original.size());
`,
		},
		{
			// Set /documents/2215/members/0,1,2,7; uniqueness and structural copies.
			name: "setConstructorsAndCloneHaveIndependentStructure",
			source: `
Set<Integer> empty = new Set<Integer>();
System.assert(empty.isEmpty());
List<Integer> sourceList = new List<Integer>{1, 1, 2};
Set<Integer> fromList = new Set<Integer>(sourceList);
System.assertEquals(2, fromList.size());
Set<Integer> copied = new Set<Integer>(fromList);
Set<Integer> cloned = fromList.clone();
sourceList.add(3);
fromList.add(4);
copied.add(5);
cloned.add(6);
System.assertEquals(false, fromList.contains(3));
System.assertEquals(false, copied.contains(4));
System.assertEquals(false, cloned.contains(4));
System.assertEquals(false, fromList.contains(5));
System.assertEquals(false, fromList.contains(6));
System.assertEquals(3, fromList.size());
System.assertEquals(3, copied.size());
System.assertEquals(3, cloned.size());
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The public List append overload is Void (/documents/2205/members/4),
// as are indexed add (/3) and addAll (/5,/6). Set add and addAll return
// changed-result Booleans (/documents/2215/members/3,/4,/5).
// Valid Apex cannot assign a Void expression. Check the production member-call
// return directly rather than introducing invalid Apex into the regression.
func TestListSetMutationReturnContractsAPI67(t *testing.T) {
	cases := []struct {
		name    string
		setup   string
		method  string
		args    []Value
		want    Value
		wantLen int
	}{
		{"listAppendVoid", "List<Integer> items = new List<Integer>{1};", "add", []Value{Int(2)}, Null, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.setup, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			machine := New(nil)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			receiver, ok := machine.Globals["items"]
			if !ok {
				t.Fatal("setup did not declare items")
			}
			got, handled, err := machine.callValueMember("items", receiver, tc.method, tc.args, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if !handled {
				t.Fatalf("%s was not handled", tc.method)
			}
			if got.Kind != tc.want.Kind || !got.Equal(tc.want) {
				t.Fatalf("return = %s (%s), want %s (%s)", got.String(), got.Kind, tc.want.String(), tc.want.Kind)
			}
			if gotLen := len(collectionMembers(machine.Globals["items"])); gotLen != tc.wantLen {
				t.Fatalf("receiver size = %d, want %d", gotLen, tc.wantLen)
			}
		})
	}
}
