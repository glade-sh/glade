package vm

import "testing"

// The retained API 67.0 source catalog SHA-256 is
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b.
// List.clear removes every element (/documents/2205/members/7), Set.clear
// removes every element (/documents/2215/members/6), and Map.clear removes
// every mapping (/documents/2207/members/3). List.clone and Set.clone make
// duplicate copies (/documents/2205/members/8 and /documents/2215/members/7).
// Collection variables refer to objects; calling a mutator changes that object:
// https://developer.salesforce.com/blogs/developer-relations/2012/05/passing-parameters-by-reference-and-by-value-in-apex
func TestExecCollectionClearPreservesAliasesAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "listAlias",
			source: `
List<Integer> original = new List<Integer>{1, 2};
List<Integer> alias = original;
original.clear();
System.assertEquals(0, original.size());
System.assertEquals(0, alias.size());
System.assert(alias.isEmpty());
alias.add(3);
System.assertEquals(1, original.size());
System.assertEquals(3, original.get(0));
alias.clear();
System.assert(original.isEmpty());
`,
		},
		{
			name: "setAlias",
			source: `
Set<Integer> original = new Set<Integer>{1, 2};
Set<Integer> alias = original;
original.clear();
System.assertEquals(0, original.size());
System.assertEquals(0, alias.size());
System.assert(alias.isEmpty());
alias.add(3);
System.assertEquals(1, original.size());
System.assert(original.contains(3));
alias.clear();
System.assert(original.isEmpty());
`,
		},
		{
			name: "listCloneRemainsIndependent",
			source: `
List<Integer> original = new List<Integer>{1, 2};
List<Integer> copy = original.clone();
original.clear();
System.assert(original.isEmpty());
System.assertEquals(2, copy.size());
System.assertEquals(1, copy.get(0));
copy.add(3);
System.assert(original.isEmpty());
System.assertEquals(3, copy.size());
`,
		},
		{
			name: "setCloneRemainsIndependent",
			source: `
Set<Integer> original = new Set<Integer>{1, 2};
Set<Integer> copy = original.clone();
original.clear();
System.assert(original.isEmpty());
System.assertEquals(2, copy.size());
System.assert(copy.contains(1));
copy.add(3);
System.assert(original.isEmpty());
System.assertEquals(3, copy.size());
`,
		},
		{
			name: "mapAliasControl",
			source: `
Map<String, Integer> original = new Map<String, Integer>{'a' => 1, 'b' => 2};
Map<String, Integer> alias = original;
original.clear();
System.assertEquals(0, original.size());
System.assertEquals(0, alias.size());
System.assert(alias.isEmpty());
System.assertEquals(null, alias.get('a'));
alias.put('c', 3);
System.assertEquals(1, original.size());
System.assertEquals(3, original.get('c'));
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
