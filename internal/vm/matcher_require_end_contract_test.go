package vm

import (
	"testing"
)

// Native contract for matcher require end contract.
// Prospective destination internal/vm/matcher_require_end_contract_test.go is UNADMITTED.
// Actual red observer remains the accepted f3 CLI. Source API67.0; REST65.0 is
// Source-established, with no direct runtime REST reporter.
// Ordered ordinary control then focal: two assertions per body, four authored total.
// Assertion reach is unknown. Diagnostic arguments and final Matcher null cleanup
// are the only approved changes to the immutable original 239-byte bodies.
func TestExecMatcherRequireEndSuccessfulSearchAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "ordinarySearchControl",
			source: `Matcher ordinary = Pattern.compile('a').matcher('a');
Boolean ordinaryFound = ordinary.find();
System.assertEquals(true, ordinaryFound, 'successful-search-end ordinary find viability');
Boolean ordinaryRequiresEnd = ordinary.requireEnd();
System.assertEquals(false, ordinaryRequiresEnd);
ordinary = null;
`,
		},
		{
			name: "endAnchoredSearchRequiresEnd",
			source: `Matcher anchored = Pattern.compile('a$').matcher('a');
Boolean anchoredFound = anchored.find();
System.assertEquals(true, anchoredFound, 'successful-search-end anchored find viability');
Boolean anchoredRequiresEnd = anchored.requireEnd();
System.assertEquals(true, anchoredRequiresEnd);
anchored = null;
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
