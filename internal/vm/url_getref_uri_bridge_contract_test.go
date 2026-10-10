package vm

import "testing"

// SOURCE ONLY: six API67 Java-URI bridge programs, uncompiled and unexecuted.
// Root accepted qualified URI inference; Query owns fixture/profile review.
// Retained Apex getRef Usage remains contradictory; direct Apex/native equivalence is open.
// Source packet18375B/SHA256ba234c4b44697769434e0d4d61dc4db2416edb3e7055b8261f198376ddc010a2.
// Fresh VM per case; no org, REST or request-context producer. Assertion reach unknown.
func TestExecURLGetRefURIBridgeAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "case01AbsentNoQuery",
			source: `URL value = new URL('https://example.test/path');
String actual = value.getRef();
System.assertEquals(null, actual);
`,
		},
		{
			name: "case02AbsentQuery",
			source: `URL value = new URL('https://example.test/path?q=1');
String actual = value.getRef();
System.assertEquals(null, actual);
`,
		},
		{
			name: "case03EmptyNoQuery",
			source: `URL value = new URL('https://example.test/path#');
String actual = value.getRef();
System.assert(actual != null && actual.equals(''));
`,
		},
		{
			name: "case04EmptyQuery",
			source: `URL value = new URL('https://example.test/path?q=1#');
String actual = value.getRef();
System.assert(actual != null && actual.equals(''));
`,
		},
		{
			name: "case05NamedNoQuery",
			source: `URL value = new URL('https://example.test/path#anchor');
String actual = value.getRef();
System.assert(actual != null && actual.equals('anchor'));
`,
		},
		{
			name: "case06NamedQuery",
			source: `URL value = new URL('https://example.test/path?q=1#anchor');
String actual = value.getRef();
System.assert(actual != null && actual.equals('anchor'));
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
