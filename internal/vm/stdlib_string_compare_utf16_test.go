package vm

import "testing"

// This API67 source inference orders valid complete Strings by 16-bit character
// values. These cases assert only comparison signs, including zero for equality.
func TestExecStringCompareToUsesUTF16OrderSignsAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "supplementaryVersusBMP",
			source: `
String supplementary = '😀';
String bmp = String.fromCharArray(new List<Integer>{57344});
System.assert(supplementary.compareTo(bmp) < 0);
System.assert(bmp.compareTo(supplementary) > 0);
System.assertEquals(0, supplementary.compareTo('😀'));
System.assertEquals(0, bmp.compareTo(String.fromCharArray(new List<Integer>{57344})));
`,
		},
		{
			name: "commonPrefix",
			source: `
String supplementary = 'a😀';
String bmp = 'a' + String.fromCharArray(new List<Integer>{57344});
System.assert(supplementary.compareTo(bmp) < 0);
System.assert(bmp.compareTo(supplementary) > 0);
System.assertEquals(0, supplementary.compareTo('a😀'));
`,
		},
		{
			name: "controls",
			source: `
String ascii = 'abc';
System.assert(ascii.compareTo('abd') < 0);
System.assert('abd'.compareTo(ascii) > 0);
System.assertEquals(0, ascii.compareTo('abc'));
System.assert('A'.compareTo('a') < 0);
String bmp = 'aΩ';
System.assert(bmp.compareTo('aω') < 0);
System.assert('aω'.compareTo(bmp) > 0);
System.assertEquals(0, bmp.compareTo('aΩ'));
System.assert('😀'.compareTo('😁') < 0);
`,
		},
		{
			name: "prefixLength",
			source: `
String ascii = 'ab';
System.assert(ascii.compareTo('abc') < 0);
System.assert('abc'.compareTo(ascii) > 0);
String supplementary = 'a😀';
System.assert(supplementary.compareTo('a😀b') < 0);
System.assert('a😀b'.compareTo(supplementary) > 0);
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
