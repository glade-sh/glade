package vm

import "testing"

// API67 EncodingUtil catalog SHA256
// 9d96c8f2472acb64d9d2860ab22ebb1e526edccf137b659c2bca441b1ce86a7b,
// /documents/640/members/2, explicitly requires NullPointerException for
// convertFromHex(null), and documents 4A4B4C -> JKL. No exception message,
// arity, non-String, or invalid-hex predicate is introduced here.
// Guide: apex/apex_classes_restful_encodingUtil.md SHA256
// 2a6239ea98ebc9db94c8cbb3774e713c74f3169348842f2e2cf598b4d130c0a6.
func TestExecEncodingUtilConvertFromHexNullContractAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "NullThrowsDocumentedType",
			source: `
Boolean caught = false;
try {
    EncodingUtil.convertFromHex(null);
} catch (NullPointerException e) {
    caught = true;
}
System.assertEquals(true, caught);
`,
		},
		{
			name:   "ValidHexControl",
			source: "System.assertEquals('JKL', EncodingUtil.convertFromHex('4A4B4C').toString());",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// API67 EncodingUtil /documents/640/members/5 defines form-safe ASCII as
// A-Z, a-z, 0-9, period, hyphen, asterisk and underscore; space becomes plus.
// Its W3C form-encoding link supplies percent escaping for unsafe characters.
// The tilde witness ignores hexadecimal letter case and qualifies UTF-8 only.
// Primary percent-escaping clause:
// https://www.w3.org/MarkUp/html-spec/html-spec_8.html#SEC8.2.1
func TestExecEncodingUtilUTF8FormSafeASCIIAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "AsteriskSafe", source: "System.assertEquals('*', EncodingUtil.urlEncode('*', 'UTF-8'));"},
		{name: "TildeEscaped", source: "System.assert(EncodingUtil.urlEncode('~', 'UTF-8').equalsIgnoreCase('%7E'));"},
		{name: "OtherSafeASCIIControl", source: "System.assertEquals('AZaz09.-_', EncodingUtil.urlEncode('AZaz09.-_', 'UTF-8'));"},
		{name: "SpacePlusControl", source: "System.assertEquals('+', EncodingUtil.urlEncode(' ', 'UTF-8'));"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
