package vm

import "testing"

// Native contract for stdlib string difference surrogate contract.
// New within-pair UTF16 positions need their own source qualification.
// Complete-character boundary coverage does not transfer to them.
// Fresh VM per body; no org, REST or request binding. Eight authored assertions,
// reach unknown. Query reviews exact fixture/profile and runner Source.
func TestExecStringDifferenceSurrogatePositionAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "sharedHighNoPrefix",
			source: `String left = '😀';
String right = '😃';
Integer actual = left.indexOfDifference(right);
System.assertEquals(1, actual);
`,
		},
		{
			name: "sharedHighAfterPrefix",
			source: `String left = 'a😀x';
String right = 'a😃y';
Integer actual = left.indexOfDifference(right);
System.assertEquals(2, actual);
`,
		},
		{
			name: "ascii",
			source: `String left = 'abcd';
String right = 'abxc';
Integer actual = left.indexOfDifference(right);
System.assertEquals(2, actual);
`,
		},
		{
			name: "bmp",
			source: `String left = 'aΩb';
String right = 'aΩc';
Integer actual = left.indexOfDifference(right);
System.assertEquals(2, actual);
`,
		},
		{
			name: "equal",
			source: `String left = 'a😀';
String right = 'a😀';
Integer actual = left.indexOfDifference(right);
System.assertEquals(-1, actual);
`,
		},
		{
			name: "completeBoundary",
			source: `String left = 'a😀b';
String right = 'a😀c';
Integer actual = left.indexOfDifference(right);
System.assertEquals(3, actual);
`,
		},
		{
			name: "shorterPrefix",
			source: `String left = 'a😀';
String right = 'a😀b';
Integer actual = left.indexOfDifference(right);
System.assertEquals(3, actual);
`,
		},
		{
			name: "differentHigh",
			source: `String left = '😀';
String right = '𝄞';
Integer actual = left.indexOfDifference(right);
System.assertEquals(0, actual);
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
