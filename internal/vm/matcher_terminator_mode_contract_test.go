package vm

import (
	"testing"
)

// Native contract for matcher terminator mode contract.
// Prospective destination internal/vm/matcher_terminator_mode_contract_test.go is UNADMITTED.
// Frozen observer is current Go sourcec262a36c; SourceAPI67/orgREST65/empty namespace.
// Ten ordered Root-frozen bodies: nine focal cases, then mandatory ordinary control.
// Twenty authored assertions; reach unknown. Both steps have exact case diagnostics.
// Fresh org/VM per subcase; focal failure leaves the final control available to run.
// Finite Salesforce Java-bridge/OpenJDK source predicates accepted separately by Root.
func TestExecMatcherTerminatorModeEndStateAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "defaultFinalCR",
			source: `Matcher candidate = Pattern.compile('a$').matcher('a\r');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes defaultFinalCR find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'terminators-and-modes defaultFinalCR requireEnd');
candidate = null;
`,
		},
		{
			name: "defaultFinalNEL",
			source: `Matcher candidate = Pattern.compile('a$').matcher('a\u0085');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes defaultFinalNEL find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'terminators-and-modes defaultFinalNEL requireEnd');
candidate = null;
`,
		},
		{
			name: "defaultFinalLS",
			source: `Matcher candidate = Pattern.compile('a$').matcher('a\u2028');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes defaultFinalLS find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'terminators-and-modes defaultFinalLS requireEnd');
candidate = null;
`,
		},
		{
			name: "defaultFinalPS",
			source: `Matcher candidate = Pattern.compile('a$').matcher('a\u2029');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes defaultFinalPS find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'terminators-and-modes defaultFinalPS requireEnd');
candidate = null;
`,
		},
		{
			name: "multilineFinalCR",
			source: `Matcher candidate = Pattern.compile('(?m)a$').matcher('a\r');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes multilineFinalCR find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(false, depends, 'terminators-and-modes multilineFinalCR requireEnd');
candidate = null;
`,
		},
		{
			name: "multilineFinalNEL",
			source: `Matcher candidate = Pattern.compile('(?m)a$').matcher('a\u0085');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes multilineFinalNEL find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(false, depends, 'terminators-and-modes multilineFinalNEL requireEnd');
candidate = null;
`,
		},
		{
			name: "multilineFinalLS",
			source: `Matcher candidate = Pattern.compile('(?m)a$').matcher('a\u2028');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes multilineFinalLS find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(false, depends, 'terminators-and-modes multilineFinalLS requireEnd');
candidate = null;
`,
		},
		{
			name: "multilineFinalPS",
			source: `Matcher candidate = Pattern.compile('(?m)a$').matcher('a\u2029');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes multilineFinalPS find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(false, depends, 'terminators-and-modes multilineFinalPS requireEnd');
candidate = null;
`,
		},
		{
			name: "uppercaseZAtEOFRequiresEnd",
			source: `Matcher candidate = Pattern.compile('a\\Z').matcher('a');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes uppercaseZAtEOFRequiresEnd find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(true, depends, 'terminators-and-modes uppercaseZAtEOFRequiresEnd requireEnd');
candidate = null;
`,
		},
		{
			name: "ordinaryFinalCRControl",
			source: `Matcher candidate = Pattern.compile('a').matcher('a\r');
Boolean found = candidate.find();
System.assertEquals(true, found, 'terminators-and-modes ordinaryFinalCRControl find viability');
Boolean depends = candidate.requireEnd();
System.assertEquals(false, depends, 'terminators-and-modes ordinaryFinalCRControl requireEnd');
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
