package vm

import "testing"

// Scratch-org API67 observations supersede the earlier ECMA standards inference.
func TestExecStringUnescapeHTML4AndEcmaScriptContractAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "html4OElig",
			source: `String input = String.fromCharArray(new List<Integer>{38,79,69,108,105,103,59});
String expected = String.fromCharArray(new List<Integer>{338});
System.assertEquals(expected, input.unescapeHtml4());
`,
		},
		{
			name: "ecmaHex41",
			source: `String input = String.fromCharArray(new List<Integer>{92,120,52,49});
String expected = String.fromCharArray(new List<Integer>{120,52,49});
System.assertEquals(expected, input.unescapeEcmaScript());
`,
		},
		{
			name: "ecmaVerticalTab",
			source: `String input = String.fromCharArray(new List<Integer>{92,118});
String expected = String.fromCharArray(new List<Integer>{118});
System.assertEquals(expected, input.unescapeEcmaScript());
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, compileErr := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if compileErr != nil {
				t.Fatalf("step=compile case=%s api=67.0 source=%q compileErr=%v", tc.name, tc.source, compileErr)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("step=program-api-guard case=%s api=67.0 source=%q got=%q want=67.0 error=compiled-api-mismatch", tc.name, tc.source, program.APIVersion)
			}
			if _, executeErr := New(nil).Execute(program); executeErr != nil {
				t.Fatalf("step=execute case=%s api=67.0 source=%q executeErr=%v", tc.name, tc.source, executeErr)
			}
		})
	}
}

func TestStringUnescapeModePreservation(t *testing.T) {
	cases := []struct {
		name   string
		method string
		input  string
		want   string
	}{
		{"html3EntityControl", "unescapeHtml3", "&OElig;", "&OElig;"},
		{"xmlEntityControl", "unescapeXml", "&OElig;", "&OElig;"},
		{"html4SinglePassControl", "unescapeHtml4", "&amp;OElig;", "&OElig;"},
		{"html4UnknownEntityControl", "unescapeHtml4", "&notarealentity;&apos;", "&notarealentity;&apos;"},
		{"javaEscapeModeControl", "unescapeJava", "\\x41\\v", "x41v"},
		{"unicodeEscapeModeControl", "unescapeUnicode", "\\x41\\v", "x41v"},
		{"ecmaEscapedSlashControl", "unescapeEcmaScript", "\\\\x41\\\\v", "\\x41\\v"},
		{"ecmaHexWidthControl", "unescapeEcmaScript", "\\x4A3", "x4A3"},
		{"ecmaMalformedFallbackControl", "unescapeEcmaScript", "\\x4\\x+1\\xGG", "x4x+1xGG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, handled, err := callStringMember(String(tc.input), tc.method, nil)
			if err != nil || !handled || got.Kind != ValueString || got.Text != tc.want {
				t.Fatalf("method=%s input=%q got=%#v handled=%v err=%v want=%q", tc.method, tc.input, got, handled, err, tc.want)
			}
		})
	}
	if got, err := unescapeJavaLike("Apex string literal", "\\x41\\v"); err != nil || got != "x41v" {
		t.Fatalf("Apex literal decoder got=%q err=%v want=x41v", got, err)
	}
}
