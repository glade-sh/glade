package vm

import (
	"fmt"
	"testing"
)

// Expected values come from the owned API 62/67/68 anonymous and @IsTest
// numeric-map observations, including distinct runtime types and Decimal scales.
func TestMapNumericValueEqualityMatchesNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, expression string
			want             bool
		}{
			{name: "mixedOperator", expression: "ints == decimals"},
			{name: "mixedEquals", expression: "ints.equals(decimals)"},
			{name: "reverseOperator", expression: "decimals == ints"},
			{name: "reverseEquals", expression: "decimals.equals(ints)"},
			{name: "integerSame", expression: "ints == new Map<String,Object>{'slot'=>1}", want: true},
			{name: "decimalSame", expression: "decimals == new Map<String,Object>{'slot'=>Decimal.valueOf('1.00')}"},
			{name: "integerSameEquals", expression: "ints.equals(new Map<String,Object>{'slot'=>1})", want: true},
			{name: "decimalSameEquals", expression: "decimals.equals(new Map<String,Object>{'slot'=>Decimal.valueOf('1.00')})"},
			{name: "scalarOperator", expression: "1 == Decimal.valueOf('1.0')", want: true},
			{name: "typedDecimalOperator", expression: "new Map<String,Decimal>{'slot'=>1} == new Map<String,Decimal>{'slot'=>Decimal.valueOf('1.0')}"},
			{name: "differentValue", expression: "ints == new Map<String,Object>{'slot'=>Decimal.valueOf('2.0')}"},
			{name: "stringCase", expression: "new Map<String,Object>{'slot'=>'north'} == new Map<String,Object>{'slot'=>'NORTH'}"},
			{name: "scalarString", expression: "'north' == 'NORTH'", want: true},
			{name: "decimalSameScale", expression: "decimals == new Map<String,Object>{'slot'=>Decimal.valueOf('1.0')}", want: true},
			{name: "decimalSameScaleEquals", expression: "decimals.equals(new Map<String,Object>{'slot'=>Decimal.valueOf('1.0')})", want: true},
			{name: "integerDecimalZero", expression: "ints == new Map<String,Object>{'slot'=>Decimal.valueOf('1')}"},
			{name: "integerDecimalZeroEquals", expression: "ints.equals(new Map<String,Object>{'slot'=>Decimal.valueOf('1')})"},
			{name: "decimalZeroScale", expression: "new Map<String,Object>{'slot'=>Decimal.valueOf('1')} == decimals"},
			{name: "scalarDecimalScale", expression: "Decimal.valueOf('1.0') == Decimal.valueOf('1.00')", want: true},
			{name: "typedDecimalSameScale", expression: "new Map<String,Decimal>{'slot'=>Decimal.valueOf('1.0')} == new Map<String,Decimal>{'slot'=>Decimal.valueOf('1.0')}", want: true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				source := `
Map<String,Object> ints = new Map<String,Object>{'slot'=>1};
Map<String,Object> decimals = new Map<String,Object>{'slot'=>Decimal.valueOf('1.0')};
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

// These are local preservation controls for existing runtime representations,
// not new native claims about Long or Double values inside collections.
func TestMapNumericValueEqualityPreservesOtherNumericKinds(t *testing.T) {
	for _, tc := range []struct {
		name        string
		left, right Value
	}{
		{name: "longDecimal", left: longIntValue(1), right: Decimal(1)},
		{name: "doubleInteger", left: decimalAsDouble(Decimal(1)), right: Int(1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			machine.Globals["leftNumber"] = tc.left
			machine.Globals["rightNumber"] = tc.right
			program, err := CompileAnonymous(`
Map<String,Object> left = new Map<String,Object>{'slot'=>leftNumber};
Map<String,Object> right = new Map<String,Object>{'slot'=>rightNumber};
System.assert(left == right);
System.assert(right == left);
System.assert(left.equals(right));
System.assert(right.equals(left));
System.assert(leftNumber == rightNumber);
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
