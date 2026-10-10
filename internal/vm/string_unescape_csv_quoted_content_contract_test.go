package vm

import "testing"

// the native compatibility contract targets the four admitted API67 CSV column cases.
// Public contract: apex_methods_system_string.md, lines 3992–3999.
// Source SHA256: 129d0369bf39b96eb2cbabed146cc1fabe766f460acc819158afaabca18865c3.
// Summer26 catalog: documents[2219].members[113], apex:System.String.unescapeCsv().
// Enclosing quotes alone do not satisfy the interior special-content condition.
func TestExecStringUnescapeCSVQuotedContentAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "quotedPlain",
			source: `
String value = '"plain"';
System.assert(value.unescapeCsv().equals('"plain"'));
`,
		},
		{
			name: "unquotedPlain",
			source: `
String value = 'plain';
System.assert(value.unescapeCsv().equals('plain'));
`,
		},
		{
			name: "quotedComma",
			source: `
String value = '"left,right"';
System.assert(value.unescapeCsv().equals('left,right'));
`,
		},
		{
			name: "doubledQuote",
			source: `
String value = '"a""b"';
System.assert(value.unescapeCsv().equals('a"b'));
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
