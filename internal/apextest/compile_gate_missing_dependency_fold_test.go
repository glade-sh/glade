package apextest

import (
	"strings"
	"testing"
	"unicode"
)

// Every rune strings.ToLower maps onto an ASCII letter must keep the source on
// the parsing path for a namespace holding that letter.
func TestCompileGateContainsFoldCoversEveryRuneLoweringToASCII(t *testing.T) {
	for r := rune(0x80); r <= unicode.MaxRune; r++ {
		lower := unicode.ToLower(r)
		if lower >= 0x80 || lower < 'a' || lower > 'z' {
			continue
		}
		name := "ns" + string(lower)
		source := "public class A { void m() { ns" + string(r) + ".Util.run(); } }"
		loweredSource := strings.ToLower(source)
		if loweredSource == source {
			t.Fatalf("rune %U does not lower", r)
		}
		if !compileGateContainsFold(source, name) {
			t.Errorf("rune %U lowers to %q but the prefilter misses namespace %q", r, lower, name)
		}
	}
}

func TestCompileGateContainsFoldKeepsNonASCIINamespaceOnParsingPath(t *testing.T) {
	// Cyrillic capital and small er differ in their first UTF-8 byte.
	if !compileGateContainsFold("public class A { рns__Foo__c f; }", "Рns") {
		t.Fatal("non-ASCII namespace must not be answered by the byte prefilter")
	}
}

func TestCompileGateReferencesMissingNamespaceFoldedRuneIdentifier(t *testing.T) {
	units := compileGateUnits{fileOf: map[string]string{}}
	withRune := "public class A { void m() { nsi.Util.run(); } }"
	dotted := strings.Replace(withRune, "nsi", "nsİ", 1)
	want := compileGateReferencesMissingNamespace("A.cls", dotted+"\n// nsi", units, []string{"nsi"})
	if got := compileGateReferencesMissingNamespace("A.cls", dotted, units, []string{"nsi"}); got != want {
		t.Fatalf("prefilter answered %v, parsing path %v", got, want)
	}
}
