package vm

import "testing"

// the native compatibility contract targets API67 MessageFormat numeric argument indices.
// Public contract: apex_methods_system_string.md, lines 1369–1380.
// Source SHA256: 129d0369bf39b96eb2cbabed146cc1fabe766f460acc819158afaabca18865c3.
func TestExecStringFormatMissingArgumentIndexAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "missingZero",
			source: `
System.assert(String.format('{00}', new List<Object>()).equals('{0}'));
`,
		},
		{
			name: "missingOne",
			source: `
System.assert(String.format('{01}', new List<Object>{'Ada'}).equals('{1}'));
`,
		},
		{
			name: "availableZero",
			source: `
System.assert(String.format('{00}', new List<Object>{'Ada'}).equals('Ada'));
`,
		},
		{
			name: "missingTypedOne",
			source: `
System.assert(String.format('{01,number}', new List<Object>{'Ada'}).equals('{1}'));
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
