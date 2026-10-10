package vm

import "testing"

// the native compatibility contract targets the four admitted API67 escapeSingleQuotes cases.
// Public contract: apex_methods_system_string.md, lines 1278–1280.
// Source SHA256: 129d0369bf39b96eb2cbabed146cc1fabe766f460acc819158afaabca18865c3.
// Summer26 catalog: documents[2219].members[29],
// apex:System.String.escapeSingleQuotes(String).
// An escape character is added before each input quote or backslash.
func TestExecStringEscapeSingleQuotesBackslashAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "standaloneBackslash",
			source: `
String value = '\\';
System.assert(String.escapeSingleQuotes(value).equals('\\\\'));
`,
		},
		{
			name: "embeddedBackslash",
			source: `
String value = 'a\\b';
System.assert(String.escapeSingleQuotes(value).equals('a\\\\b'));
`,
		},
		{
			name: "quoteControl",
			source: `
String value = 'Bob\'s';
System.assert(String.escapeSingleQuotes(value).equals('Bob\\\'s'));
`,
		},
		{
			name: "plainControl",
			source: `
String value = 'plain';
System.assert(String.escapeSingleQuotes(value).equals('plain'));
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
