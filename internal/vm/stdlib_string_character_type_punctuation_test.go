package vm

import "testing"

// The admitted API67 cases distinguish Unicode dash punctuation (Pd) from
// other punctuation (Po), with existing character groups preserved.
func TestExecStringCharacterTypeSeparatesDashAndOtherPunctuationAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "splitDashAndDot",
			source: `
List<String> parts = '-.'.splitByCharacterType();
System.assertEquals(2, parts.size());
System.assert(parts.get(0).equals('-'));
System.assert(parts.get(1).equals('.'));
`,
		},
		{
			name: "splitCamelCaseDashAndDot",
			source: `
List<String> parts = '-.'.splitByCharacterTypeCamelCase();
System.assertEquals(2, parts.size());
System.assert(parts.get(0).equals('-'));
System.assert(parts.get(1).equals('.'));
`,
		},
		{
			name: "sameDashAndCharacterGroupControls",
			source: `
List<String> parts = '--'.splitByCharacterType();
System.assertEquals(1, parts.size());
System.assert(parts.get(0).equals('--'));
List<String> groups = 'ab12 CD'.splitByCharacterType();
System.assertEquals(4, groups.size());
System.assert(groups.get(0).equals('ab'));
System.assert(groups.get(1).equals('12'));
System.assert(groups.get(2).equals(' '));
System.assert(groups.get(3).equals('CD'));
`,
		},
		{
			name: "sameDotAndCamelCaseControls",
			source: `
List<String> parts = '..'.splitByCharacterTypeCamelCase();
System.assertEquals(1, parts.size());
System.assert(parts.get(0).equals('..'));
List<String> groups = 'HTTPServer42'.splitByCharacterTypeCamelCase();
System.assertEquals(3, groups.size());
System.assert(groups.get(0).equals('HTTP'));
System.assert(groups.get(1).equals('Server'));
System.assert(groups.get(2).equals('42'));
`,
		},
	} {
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
