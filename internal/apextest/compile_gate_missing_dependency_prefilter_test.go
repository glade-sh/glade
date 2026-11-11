package apextest

import (
	"strings"
	"testing"
)

func TestCompileGateContainsFold(t *testing.T) {
	cases := []struct {
		source, name string
		want         bool
	}{
		{"public class A { depns__Foo__c f; }", "depns", true},
		{"public class A { DEPNS__Foo__c f; }", "depns", true},
		{"public class A { Depns.Util.run(); }", "DEPNS", true},
		{"public class A { String s = 'x'; }", "depns", false},
		{"", "depns", false},
		{"depn", "depns", false},
		{"public class A { Kelvin__Foo__c f; }", "kelvin", true},
		{"public class A { Kelvin__Foo__c f; }", "depns", false},
		{"anything", "", true},
	}
	for _, tc := range cases {
		if got := compileGateContainsFold(tc.source, tc.name); got != tc.want {
			t.Errorf("compileGateContainsFold(%q, %q) = %v, want %v", tc.source, tc.name, got, tc.want)
		}
	}
}

// The mention prefilter must answer exactly as the parsing path does: a file
// that never spells the namespace has no reference, and a file that spells it
// only inside a comment or string still has none once parsed.
func TestCompileGateReferencesMissingNamespacePrefilterMatchesParse(t *testing.T) {
	units := compileGateUnits{fileOf: map[string]string{}, types: nil, namespace: ""}
	namespaces := []string{"depns"}
	cases := []struct {
		name, source string
		want         bool
	}{
		{"no mention", "public class A { Integer i = 1; }", false},
		{"comment only", "public class A { // depns.Util\n Integer i = 1; }", false},
		{"string only", "public class A { String s = 'depns__Obj__c'; }", false},
		{"sobject field", "public class A { depns__Obj__c r = new depns__Obj__c(); }", true},
		{"sobject field upper", "public class A { DEPNS__Obj__c r = new DEPNS__Obj__c(); }", true},
		{"namespaced class", "public class A { void m() { depns.Util.run(); } }", true},
		{"namespaced class mixed case", "public class A { void m() { DepNs.Util.run(); } }", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compileGateReferencesMissingNamespace("A.cls", tc.source, units, namespaces)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			// The parsing path alone, with the prefilter forced open, agrees.
			forced := compileGateReferencesMissingNamespace("A.cls", tc.source+"\n// "+strings.Join(namespaces, " "), units, namespaces)
			if forced != tc.want {
				t.Fatalf("parsing path got %v, want %v", forced, tc.want)
			}
		})
	}
}
