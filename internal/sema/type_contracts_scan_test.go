package sema

import (
	"reflect"
	"testing"
)

var typeContractScanSamples = []string{
	"",
	"Integer x = 12345678901;",
	"Integer x = 1234567890; Long y = 123456789L;",
	"Decimal d = 1e10; Decimal e = 2.5E-3; Decimal f = 1e+2;",
	"String s = 'e1'; Integer e = 1; x = 1 e 2; y = 1e; z = e1; w = 1e+;",
	"List<String> l = new List(); Map m = new Map (  ); Set s = new SET();",
	"List<String> l = new List<String>(); foo(); new Lis(); new Mapp();",
	"new liſt(); new Keep();",
	"new ſet()",
	"acct?.Name = 'x'; acct?.Owner?.Name == 'y'; a ? b : c; a?.b();",
	"x = 1234567890123456789012345678901234567890;",
	"version 9999999999.0 and 0.9999999999",
	"1e5e5 1.e5 .5e5 1.5e5.5",
	"map () set\t( ) list\n(\n)",
	"newmap() new map() new  map () NEW Map()",
}

// TestTypeContractScansMatchPatterns pins each gated scan to the pattern it
// gates on hand-written edge cases and on every Apex source in the repository
// (plus GLADE_SEMA_EQUIV_DIRS).
func TestTypeContractScansMatchPatterns(t *testing.T) {
	check := func(t *testing.T, name, source string) {
		t.Helper()
		for _, scan := range []struct {
			name    string
			pattern func(string) [][]int
			gated   func(string) [][]int
		}{
			{"safe assignment", func(s string) [][]int { return typeContractSafeAssignment.FindAllStringIndex(s, -1) }, typeContractSafeAssignmentMatches},
			{"integer literal", func(s string) [][]int { return typeContractIntegerLiteral.FindAllStringIndex(s, -1) }, typeContractIntegerLiteralMatches},
			{"scientific literal", func(s string) [][]int { return typeContractScientificLiteral.FindAllStringIndex(s, -1) }, typeContractScientificLiteralMatches},
			{"raw collection constructor", func(s string) [][]int { return typeContractRawCollectionConstructor.FindAllStringIndex(s, -1) }, typeContractRawCollectionConstructorMatches},
		} {
			want := scan.pattern(source)
			got := scan.gated(source)
			if len(want) == 0 && len(got) == 0 {
				continue
			}
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("%s: %s matches differ\nwant %v\ngot  %v", name, scan.name, want, got)
			}
		}
	}
	for i, source := range typeContractScanSamples {
		check(t, "sample "+string(rune('A'+i)), source)
	}
	files := 0
	forEachRepositoryApexSource(t, func(path, source string) {
		files++
		check(t, path, source)
	})
	t.Logf("compared %d files", files)
}
