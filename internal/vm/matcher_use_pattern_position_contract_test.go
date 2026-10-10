package vm

import "testing"

// the native compatibility contract targets four admitted API67 ASCII Matcher cursor cases.
// Public contract: apex_classes_pattern_and_matcher_matcher_methods.md,
// usePattern lines 644–647, find lines 125–128, and start lines 578–579.
// Source SHA256: c7de2baea5f661ec1cfbba49e4b5630df66f23cf6be52ee49a52af5bbe9434f2.
// Summer26 catalog: documents[635].members[25], [2], and [22].
// usePattern preserves the search position while clearing prior group state.
func TestExecMatcherUsePatternPreservesSearchPositionAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "samePatternContinues",
			source: `
Matcher m = Pattern.compile('a').matcher('aba');
System.assert(m.find());
System.assertEquals(0, m.start());
m.usePattern(Pattern.compile('a'));
System.assert(m.find());
System.assertEquals(2, m.start());
m = null;
`,
		},
		{
			name: "changedPatternContinues",
			source: `
Matcher m = Pattern.compile('a').matcher('bab');
System.assert(m.find());
System.assertEquals(1, m.start());
m.usePattern(Pattern.compile('b'));
System.assert(m.find());
System.assertEquals(2, m.start());
m = null;
`,
		},
		{
			name: "unchangedPatternControl",
			source: `
Matcher m = Pattern.compile('a').matcher('aba');
System.assert(m.find());
System.assertEquals(0, m.start());
System.assert(m.find());
System.assertEquals(2, m.start());
m = null;
`,
		},
		{
			name: "freshPatternControl",
			source: `
Matcher m = Pattern.compile('a').matcher('bab');
m.usePattern(Pattern.compile('b'));
System.assert(m.find());
System.assertEquals(0, m.start());
m = null;
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
