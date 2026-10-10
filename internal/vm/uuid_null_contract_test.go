package vm

import "testing"

// Typed null fromString must throw a catchable NullPointerException at API67.
// Root owns exact final HEAD/tree binding, admission, installation and validation.
// Frozen proposal: /tmp/glade-uuid-null-source-proposal.json, 16334 bytes,
// SHA256 de77feebc2c005b6a6c14fc4e55657b4162b3d62f66eb19583d76e0da31f0fca.
// Retained UUID primary: 3614 bytes, SHA256
// f9fda4b39bd20c3810dfe19e1dedca09cd963e5173bea41f704b1f85f3ef8b2a.
// Exact control: lines 53-57 and 81-82. Null exception rule: line 83.
// Catalog /documents/386/members/1 fromString(String), member/0 equals(Object).
// Exactly two literal programs and three Apex assertions: one equality behavior
// control, one typed-null construction assertion before try, one specific NPE
// catch assertion. No message, malformed, formatting, RNG or hash predicates.
// API67 is the requested compile target; no API interval/native/AC7 credit.
func TestExecUUIDFromStringNullAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "uuid-documented-well-formed-equality-control", source: `String uuidText = '707b2538-98bb-41e7-95e3-1d77bf42b102';
UUID first = UUID.fromString(uuidText);
UUID second = UUID.fromString(uuidText);
System.assert(first.equals(second));`},
		{name: "uuid-typed-null-string-specific-npe", source: `String uuidText = null;
System.assert(uuidText == null);
Boolean caughtNullPointer = false;
try {
    UUID.fromString(uuidText);
} catch (NullPointerException expected) {
    caughtNullPointer = true;
}
System.assert(caughtNullPointer);`},
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
			machine := New(nil)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
