package vm

import "testing"

func TestClassLookupQualifiedKeyEqualsMatchesConcatenation(t *testing.T) {
	parts := []string{"", "ns", "NS", " ns", "ns ", "Ns\t", "a.b", "Pkg", "pkg_x", " ns", "ns ", "Util", "util", " Util", "Util "}
	keys := []string{"", "ns.util", "pkg.util", "ns. util", "a.b.util", "ns.util ", "pkg_x.util", "ns", ".util", "ns.ns"}
	for _, ns := range parts {
		for _, name := range parts {
			for _, key := range keys {
				want := classLookupKeyEquals(ns+"."+name, key)
				if ns == "" {
					continue
				}
				if got := classLookupQualifiedKeyEquals(ns, name, key); got != want {
					t.Fatalf("namespace %q name %q key %q: got %v want %v", ns, name, key, got, want)
				}
			}
		}
	}
}
