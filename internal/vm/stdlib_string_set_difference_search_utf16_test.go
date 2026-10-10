package vm

import "testing"

// The API67 source inference joins these zero-based positions to String.length's
// 16-bit character units. All matching and divergence positions are complete characters.
func TestExecStringSetAndDifferenceSearchUseUTF16PositionsAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "indexOfAny",
			source: `
String ascii = 'abc';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(2, ascii.indexOfAny('c'));
System.assertEquals(2, bmp.indexOfAny('b'));
System.assertEquals(-1, supplementary.indexOfAny('z'));
System.assertEquals(0, supplementary.indexOfAny('a'));
System.assertEquals(0, '😀b'.indexOfAny('😀'));
System.assertEquals(1, 'a😀'.indexOfAny('😀'));
System.assertEquals(3, supplementary.indexOfAny('b'), 'indexOfAny result follows the UTF-16 position after the supplementary character');
`,
		},
		{
			name: "indexOfAnyBut",
			source: `
String ascii = 'abc';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(2, ascii.indexOfAnyBut('ab'));
System.assertEquals(2, bmp.indexOfAnyBut('aΩ'));
System.assertEquals(-1, supplementary.indexOfAnyBut('a😀b'));
System.assertEquals(0, supplementary.indexOfAnyBut('😀b'));
System.assertEquals(0, '😀b'.indexOfAnyBut('b'));
System.assertEquals(1, 'a😀'.indexOfAnyBut('a'));
System.assertEquals(3, supplementary.indexOfAnyBut('a😀'), 'indexOfAnyBut result follows the UTF-16 position after the supplementary character');
`,
		},
		{
			name: "indexOfDifference",
			source: `
String ascii = 'abc';
String bmp = 'aΩb';
String supplementary = 'a😀b';
System.assertEquals(2, ascii.indexOfDifference('abd'));
System.assertEquals(2, bmp.indexOfDifference('aΩc'));
System.assertEquals(-1, supplementary.indexOfDifference('a😀b'));
System.assertEquals(0, supplementary.indexOfDifference('c😀b'));
System.assertEquals(2, '😀b'.indexOfDifference('😀c'));
System.assertEquals(3, 'a😀'.indexOfDifference('a😀b'));
System.assertEquals(3, supplementary.indexOfDifference('a😀'));
System.assertEquals(3, supplementary.indexOfDifference('a😀c'), 'indexOfDifference result follows the UTF-16 position after the supplementary character');
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
