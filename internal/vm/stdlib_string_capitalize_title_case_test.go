package vm

import "testing"

// String.capitalize's API67 contract specifies title case for the first letter.
// These BMP digraphs have distinct single-character upper and title-case forms.
func TestExecStringCapitalizeUsesTitleCaseAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "dz",
			source: `
String actual = 'ǳmIxEd'.capitalize();
System.assertEquals(498, actual.charAt(0));
System.assert(actual.equals('ǲmIxEd'));
String existing = 'ǲmIxEd';
System.assert(existing.capitalize().equals(existing));
`,
		},
		{
			name: "dzWithCaron",
			source: `
String actual = 'ǆmIxEd'.capitalize();
System.assertEquals(453, actual.charAt(0));
System.assert(actual.equals('ǅmIxEd'));
String existing = 'ǅmIxEd';
System.assert(existing.capitalize().equals(existing));
`,
		},
		{
			name: "lj",
			source: `
String actual = 'ǉmIxEd'.capitalize();
System.assertEquals(456, actual.charAt(0));
System.assert(actual.equals('ǈmIxEd'));
String existing = 'ǈmIxEd';
System.assert(existing.capitalize().equals(existing));
`,
		},
		{
			name: "nj",
			source: `
String actual = 'ǌmIxEd'.capitalize();
System.assertEquals(459, actual.charAt(0));
System.assert(actual.equals('ǋmIxEd'));
String existing = 'ǋmIxEd';
System.assert(existing.capitalize().equals(existing));
`,
		},
		{
			name: "controls",
			source: `
String ascii = 'hello mIxEd';
System.assert(ascii.capitalize().equals('Hello mIxEd'));
String existing = 'Hello mIxEd';
System.assert(existing.capitalize().equals(existing));
String bmp = 'ωmIxEd';
System.assert(bmp.capitalize().equals('ΩmIxEd'));
String empty = '';
System.assert(empty.capitalize().equals(''));
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
