package vm

import "testing"

// UUID parsing errors must remain catchable at the public API boundary.
func TestExecUUIDInequalityAndMalformedCatchAPI67(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{name: "uuid-canonical-string-inequality-control", source: `String uuidText = '707b2538-98bb-41e7-95e3-1d77bf42b102';
UUID parsed = UUID.fromString(uuidText);
System.assert(!parsed.equals(uuidText));`},
		{name: "uuid-distinct-canonical-inequality-control", source: `String firstText = '707b2538-98bb-41e7-95e3-1d77bf42b102';
String secondText = '707b2538-98bb-41e7-95e3-1d77bf42b103';
UUID first = UUID.fromString(firstText);
UUID second = UUID.fromString(secondText);
System.assert(!first.equals(second));`},
		{name: "uuid-malformed-string-specific-iae", source: `String uuidText = 'not a uuid';
Boolean caughtIllegalArgument = false;
try {
    UUID.fromString(uuidText);
} catch (IllegalArgumentException expected) {
    caughtIllegalArgument = true;
}
System.assert(caughtIllegalArgument);`},
	}
	for _, tc := range cases {
		if !t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}
			org := testDataOrg()
			org.APIVersion = "65.0"
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		}) {
			// A failed negative control stops before the malformed focal.
			return
		}
	}
}
