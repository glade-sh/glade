package vm

import "testing"

// The retained API 67.0 source catalog SHA-256 is
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b.
// Set.removeAll(List<Object>), removeAll(Set<Object>), retainAll(List<Object>),
// and retainAll(Set<Object>) are /documents/2215/members/15-18. Each mutates
// the calling set; clone makes an independent duplicate (/members/7).
// Non-primitive Apex variables retain object references, so mutator calls are
// visible through aliases:
// https://developer.salesforce.com/blogs/developer-relations/2012/05/passing-parameters-by-reference-and-by-value-in-apex
func TestExecSetBulkMutationsPreserveAliasesAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "removeAllListAlias",
			source: `
Set<Integer> original = new Set<Integer>{1, 2, 3};
Set<Integer> alias = original;
Set<Integer> copy = original.clone();
Boolean changed = original.removeAll(new List<Integer>{1});
System.assertEquals(true, changed);
System.assertEquals(2, original.size());
System.assertEquals(2, alias.size());
System.assert(!alias.contains(1));
System.assert(alias.contains(2));
System.assert(alias.contains(3));
System.assertEquals(3, copy.size());
System.assert(copy.contains(1));
alias.add(4);
System.assertEquals(3, original.size());
System.assert(original.contains(4));
System.assertEquals(3, copy.size());
System.assert(!copy.contains(4));
`,
		},
		{
			name: "removeAllSetAlias",
			source: `
Set<Integer> original = new Set<Integer>{1, 2, 3};
Set<Integer> alias = original;
Set<Integer> copy = original.clone();
Boolean changed = original.removeAll(new Set<Integer>{1});
System.assertEquals(true, changed);
System.assertEquals(2, original.size());
System.assertEquals(2, alias.size());
System.assert(!alias.contains(1));
System.assert(alias.contains(2));
System.assert(alias.contains(3));
System.assertEquals(3, copy.size());
System.assert(copy.contains(1));
alias.add(4);
System.assertEquals(3, original.size());
System.assert(original.contains(4));
System.assertEquals(3, copy.size());
System.assert(!copy.contains(4));
`,
		},
		{
			name: "retainAllListAlias",
			source: `
Set<Integer> original = new Set<Integer>{1, 2, 3};
Set<Integer> alias = original;
Set<Integer> copy = original.clone();
Boolean changed = original.retainAll(new List<Integer>{1, 3});
System.assertEquals(true, changed);
System.assertEquals(2, original.size());
System.assertEquals(2, alias.size());
System.assert(!alias.contains(2));
System.assert(alias.contains(1));
System.assert(alias.contains(3));
System.assertEquals(3, copy.size());
System.assert(copy.contains(2));
alias.add(4);
System.assertEquals(3, original.size());
System.assert(original.contains(4));
System.assertEquals(3, copy.size());
System.assert(!copy.contains(4));
`,
		},
		{
			name: "retainAllSetAlias",
			source: `
Set<Integer> original = new Set<Integer>{1, 2, 3};
Set<Integer> alias = original;
Set<Integer> copy = original.clone();
Boolean changed = original.retainAll(new Set<Integer>{1, 3});
System.assertEquals(true, changed);
System.assertEquals(2, original.size());
System.assertEquals(2, alias.size());
System.assert(!alias.contains(2));
System.assert(alias.contains(1));
System.assert(alias.contains(3));
System.assertEquals(3, copy.size());
System.assert(copy.contains(2));
alias.add(4);
System.assertEquals(3, original.size());
System.assert(original.contains(4));
System.assertEquals(3, copy.size());
System.assert(!copy.contains(4));
`,
		},
		{
			name: "mapPutAliasControl",
			source: `
Map<String, Integer> original = new Map<String, Integer>{'a' => 1};
Map<String, Integer> alias = original;
alias.put('b', 2);
System.assertEquals(2, original.size());
System.assertEquals(2, original.get('b'));
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
