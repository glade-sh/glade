package vm

import "testing"

// These API67 cases join two-argument search bounds and results to String's
// 16-bit character units, using complete-character bounds and nonempty BMP targets.
func TestExecStringCharAndIgnoreCaseTwoArgumentSearchUseUTF16PositionsAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "indexOfChar",
			source: `
String ascii = 'abcab';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(4, ascii.indexOfChar(98, 2));
System.assertEquals(2, bmp.indexOfChar(98, 2));
System.assertEquals(1, bmp.indexOfChar(937, 1));
System.assertEquals(3, supplementary.indexOfChar(98, 3));
System.assertEquals(-1, supplementary.indexOfChar(122, 3));
System.assertEquals(3, supplementary.indexOfChar(98));
`,
		},
		{
			name: "lastIndexOfChar",
			source: `
String ascii = 'abcab';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(1, ascii.lastIndexOfChar(98, 3));
System.assertEquals(2, bmp.lastIndexOfChar(98, 2));
System.assertEquals(1, bmp.lastIndexOfChar(937, 1));
System.assertEquals(3, supplementary.lastIndexOfChar(98, 3));
System.assertEquals(-1, supplementary.lastIndexOfChar(122, 3));
System.assertEquals(3, supplementary.lastIndexOfChar(98));
`,
		},
		{
			name: "indexOfIgnoreCase",
			source: `
String ascii = 'abcab';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(4, ascii.indexOfIgnoreCase('B', 2));
System.assertEquals(2, bmp.indexOfIgnoreCase('B', 2));
System.assertEquals(1, bmp.indexOfIgnoreCase('Ω', 1));
System.assertEquals(3, supplementary.indexOfIgnoreCase('B', 3));
System.assertEquals(-1, supplementary.indexOfIgnoreCase('z', 3));
System.assertEquals(3, supplementary.indexOfIgnoreCase('B'));
`,
		},
		{
			name: "lastIndexOfIgnoreCase",
			source: `
String ascii = 'abcab';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(1, ascii.lastIndexOfIgnoreCase('B', 3));
System.assertEquals(2, bmp.lastIndexOfIgnoreCase('B', 2));
System.assertEquals(1, bmp.lastIndexOfIgnoreCase('Ω', 1));
System.assertEquals(3, supplementary.lastIndexOfIgnoreCase('B', 3));
System.assertEquals(-1, supplementary.lastIndexOfIgnoreCase('z', 3));
System.assertEquals(3, supplementary.lastIndexOfIgnoreCase('B'));
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
