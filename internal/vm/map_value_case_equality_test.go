package vm

import (
	"fmt"
	"testing"
)

// Native API 62/67/68 anonymous and @IsTest observations are exported in
// internal/apextest/testdata/conformance/value_capture_native.tsv.

func TestMapStringValueEqualityPreservesCollectionCase(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name       string
			expression string
			want       bool
		}{
			{name: "sameOperator", expression: "lower == same", want: true},
			{name: "sameEquals", expression: "lower.equals(same)", want: true},
			{name: "mixedOperator", expression: "lower == mixed"},
			{name: "mixedEquals", expression: "lower.equals(mixed)"},
			{name: "boxedMixedOperator", expression: "boxedLower == boxedMixed"},
			{name: "changedOperator", expression: "lower == changed"},
			{name: "keyMixedOperator", expression: "lower == keyMixed"},
			{
				name:       "nestedMapMixed",
				expression: "new Map<String,Object>{'child' => lower} == new Map<String,Object>{'child' => mixed}",
			},
			{
				name:       "nestedListMixed",
				expression: "new Map<String,Object>{'child' => new List<String>{'north'}} == new Map<String,Object>{'child' => new List<String>{'NORTH'}}",
			},
			{name: "listMixed", expression: "new List<String>{'north'} == new List<String>{'NORTH'}"},
			{name: "scalarOperator", expression: "'north' == 'NORTH'", want: true},
			{name: "scalarEquals", expression: "'north'.equals('NORTH')"},
			{
				name:       "typedStringMixed",
				expression: "new Map<String,String>{'slot' => 'north'} == new Map<String,String>{'slot' => 'NORTH'}",
			},
			{
				name:       "nullSame",
				expression: "new Map<String,Object>{'slot' => null} == new Map<String,Object>{'slot' => null}",
				want:       true,
			},
			{name: "nullMissing", expression: "new Map<String,Object>{'slot' => null} == new Map<String,Object>()"},
			{
				name:       "booleanSame",
				expression: "new Map<String,Object>{'slot' => true} == new Map<String,Object>{'slot' => true}",
				want:       true,
			},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := `
Map<String,Object> lower = new Map<String,Object>{'slot' => 'north'};
Map<String,Object> same = new Map<String,Object>{'slot' => 'north'};
Map<String,Object> mixed = new Map<String,Object>{'slot' => 'NORTH'};
Map<String,Object> keyMixed = new Map<String,Object>{'SLOT' => 'north'};
Map<String,Object> changed = new Map<String,Object>{'slot' => 'south'};
Object boxedLower = lower;
Object boxedMixed = mixed;
` + fmt.Sprintf("System.assertEquals(%t, (%s));", tc.want, tc.expression)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := Execute(program, nil); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// This is a local preservation control for string-backed Id provenance, not
// a new claim about native mixed Id/String collection equality.
func TestMapStringValueEqualityPreservesStringBackedIdFallback(t *testing.T) {
	machine := New(nil)
	machine.Globals["typedId"] = Value{Kind: ValueString, Type: "Id", Text: "001000000000001"}
	machine.Globals["ordinaryText"] = String("001000000000001AAA")
	program, err := CompileAnonymous(`
Map<String,Object> left=new Map<String,Object>{'slot'=>typedId};
Map<String,Object> right=new Map<String,Object>{'slot'=>ordinaryText};
System.assert(left==right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
