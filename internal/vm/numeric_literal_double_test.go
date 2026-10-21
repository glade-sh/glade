package vm

import "testing"

// The Apex Developer Guide's Data Types and Variables section documents a
// Double literal with a D suffix; its defaultValue section specifies that the
// suffix is case-insensitive. Double is a 64-bit floating-point value.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf
func TestParseDoubleSuffixedNumericLiteral(t *testing.T) {
	for _, literal := range []string{"42.0d", "42.0D", "0.0d", "2.7182818284D"} {
		t.Run(literal, func(t *testing.T) {
			value, err := parseLiteral(literal)
			if err != nil {
				t.Fatal(err)
			}
			if value.Kind != ValueDecimal || value.Runtime != "Double" || value.Static != "Double" {
				t.Fatalf("literal %s lost its Double identity: %#v", literal, value)
			}
		})
	}
}

func TestDoubleSuffixedNumericLiteralUsesFloatArithmetic(t *testing.T) {
	for _, tc := range []struct {
		literal string
		text    string
	}{
		{literal: "9007199254740992.0d", text: "9007199254740992"},
		{literal: "9007199254740992.0D", text: "9007199254740992"},
		{literal: "9007199254740992.0", text: "9007199254740993.0"},
	} {
		t.Run(tc.literal, func(t *testing.T) {
			value, err := parseLiteral(tc.literal)
			if err != nil {
				t.Fatal(err)
			}
			result, err := evalBinary("+", value, Int(1))
			if err != nil {
				t.Fatal(err)
			}
			if result.Text != tc.text {
				t.Fatalf("%s + 1 = %q, want %q", tc.literal, result.Text, tc.text)
			}
		})
	}
}
