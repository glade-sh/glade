package vm

import "testing"

// The API67 source inference counts single-character edits in 16-bit units.
// All inputs are complete Strings; thresholds apply to empty targets as well.
func TestExecStringLevenshteinUsesUTF16UnitsAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "supplementaryToEmpty",
			source: `
String supplementary = '😀';
System.assertEquals(2, supplementary.getLevenshteinDistance(''));
`,
		},
		{
			name: "belowThreshold",
			source: `
String supplementary = '😀';
System.assertEquals(-1, supplementary.getLevenshteinDistance('', 1));
`,
		},
		{
			name: "atThreshold",
			source: `
String supplementary = '😀';
System.assertEquals(2, supplementary.getLevenshteinDistance('', 2));
`,
		},
		{
			name: "controls",
			source: `
String ascii = 'a';
System.assertEquals(1, ascii.getLevenshteinDistance(''));
String bmp = 'Ω';
System.assertEquals(1, bmp.getLevenshteinDistance(''));
String supplementary = '😀';
System.assertEquals(0, supplementary.getLevenshteinDistance('😀'));
String empty = '';
System.assertEquals(0, empty.getLevenshteinDistance(''));
System.assertEquals(0, ascii.getLevenshteinDistance('a', 0));
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
