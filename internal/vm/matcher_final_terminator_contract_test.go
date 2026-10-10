package vm

import (
	"testing"
)

// Native contract for matcher final terminator contract.
// Prospective destination internal/vm/matcher_final_terminator_contract_test.go is UNADMITTED.
// Frozen observer is current Go source6630c700; SourceAPI67/orgREST65/empty namespace.
// Three ordered bodies: default LF, default CRLF, then mandatory multiline LF control.
// Six authored assertions; reach unknown. Both steps have distinct case diagnostics.
// Fresh org/VM per subcase; focal failure leaves the final control available to run.
// Predicates are bounded Salesforce Java-bridge/OpenJDK source inferences.
func TestExecMatcherFinalTerminatorEndStateAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "finalLFDefaultRequiresEnd",
			source: `Matcher candidate = Pattern.compile('a$').matcher('a\n');
Boolean found = candidate.find();
System.assertEquals(true, found, 'final-terminators finalLFDefaultRequiresEnd find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'final-terminators finalLFDefaultRequiresEnd requireEnd');
candidate = null;
`,
		},
		{
			name: "finalCRLFDefaultRequiresEnd",
			source: `Matcher candidate = Pattern.compile('a$').matcher('a\r\n');
Boolean found = candidate.find();
System.assertEquals(true, found, 'final-terminators finalCRLFDefaultRequiresEnd find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'final-terminators finalCRLFDefaultRequiresEnd requireEnd');
candidate = null;
`,
		},
		{
			name: "multilineFinalLFControl",
			source: `Matcher candidate = Pattern.compile('(?m)a$').matcher('a\n');
Boolean found = candidate.find();
System.assertEquals(true, found, 'final-terminators multilineFinalLFControl find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(false, depends, 'final-terminators multilineFinalLFControl requireEnd');
candidate = null;
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

			org := testDataOrg()
			org.APIVersion = "65.0"
			org.Namespace = ""
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
