package vm

import "testing"

// Source-only fixture; unexecuted.
// Blank classification follows the documented whitespace, empty, or null rule.
// Eight authored assertions; reach unknown.
func TestExecStringLiteralBlankContractAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "recordTypeNameBlank",
			source: `String text = '$RecordType.Name';
Boolean actual = String.isBlank(text);
System.assertEquals(false, actual);
`,
		},
		{
			name: "recordTypeNameNotBlank",
			source: `String text = '$RecordType.Name';
Boolean actual = String.isNotBlank(text);
System.assertEquals(true, actual);
`,
		},
		{
			name: "recordTypeDeveloperNameBlank",
			source: `String text = '$RecordType.DeveloperName';
Boolean actual = String.isBlank(text);
System.assertEquals(false, actual);
`,
		},
		{
			name: "recordTypeDeveloperNameNotBlank",
			source: `String text = '$RecordType.DeveloperName';
Boolean actual = String.isNotBlank(text);
System.assertEquals(true, actual);
`,
		},
		{
			name: "nullControl",
			source: `String text = null;
Boolean actual = String.isBlank(text);
System.assertEquals(true, actual);
`,
		},
		{
			name: "emptyControl",
			source: `String text = '';
Boolean actual = String.isNotBlank(text);
System.assertEquals(false, actual);
`,
		},
		{
			name: "whitespaceControl",
			source: `String text = '  ';
Boolean actual = String.isBlank(text);
System.assertEquals(true, actual);
`,
		},
		{
			name: "ordinaryControl",
			source: `String text = 'Hello';
Boolean actual = String.isNotBlank(text);
System.assertEquals(true, actual);
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, compileErr := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if compileErr != nil {
				t.Fatalf("step=compile case=%s api=67.0 source=%q compileErr=%v", tc.name, tc.source, compileErr)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("step=program-api-guard case=%s api=67.0 source=%q got=%q want=67.0 error=compiled-api-mismatch", tc.name, tc.source, program.APIVersion)
			}
			machine := New(nil)
			if _, executeErr := machine.Execute(program); executeErr != nil {
				t.Fatalf("step=execute case=%s api=67.0 source=%q executeErr=%v", tc.name, tc.source, executeErr)
			}
		})
	}
}
