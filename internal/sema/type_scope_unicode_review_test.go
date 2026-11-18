package sema

import "testing"

// The declaration patterns use case-insensitive RE2 character classes, which
// include Unicode simple folds of ASCII letters in annotation identifiers.
func TestTypeScopeFieldBindingsUnicodeAnnotationMatchesFullScan(t *testing.T) {
	for _, source := range []string{
		"class A { @\u017f(x={f()}) static Integer value; }",
		"class A { @\u212a(x={f()}) static String value; }",
	} {
		assertTypeScopeFieldBindingsEquivalent(t, "Unicode annotation", source)
	}
}
