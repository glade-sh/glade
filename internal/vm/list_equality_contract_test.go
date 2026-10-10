package vm

import "testing"

// API 67.0 documents List.equals(List) as equivalent to == using ordered
// comparison (Salesforce docs inventory SHA-256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/2205/members/11). These checks establish that equivalence without
// assuming either operator's native Boolean result for any pair.
func TestExecListEqualsMatchesOrderedComparisonAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "stringCaseVariant",
			source: `
List<String> left = new List<String>{'a'};
List<String> right = new List<String>{'A'};
System.assertEquals(left == right, left.equals(right));
`,
		},
		{
			name: "identicalValues",
			source: `
List<Integer> left = new List<Integer>{1, 2};
List<Integer> right = new List<Integer>{1, 2};
System.assertEquals(left == right, left.equals(right));
`,
		},
		{
			name: "differentLength",
			source: `
List<Integer> left = new List<Integer>{1, 2};
List<Integer> right = new List<Integer>{1};
System.assertEquals(left == right, left.equals(right));
`,
		},
		{
			name: "reorderedValues",
			source: `
List<Integer> left = new List<Integer>{1, 2};
List<Integer> right = new List<Integer>{2, 1};
System.assertEquals(left == right, left.equals(right));
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
