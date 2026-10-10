package vm

import (
	"fmt"
	"testing"
)

func TestNullDecimalArithmeticAPI40(t *testing.T) {
	for _, test := range []struct{ name, expression string }{
		{"AdditionLeft", "value + 3"},
		{"AdditionRight", "3 + value"},
		{"SubtractionLeft", "value - 3"},
		{"SubtractionRight", "3 - value"},
		{"MultiplicationLeft", "value * 3"},
		{"MultiplicationRight", "3 * value"},
		{"DivisionLeft", "value / 3"},
		{"DivisionRight", "3 / value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`Decimal value;String observed;try{Decimal result=%s;observed='value:'+String.valueOf(result);}catch(Exception e){observed='error:'+e.getTypeName();}System.assertEquals('error:System.NullPointerException',observed);`, test.expression)
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: "40.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Method bodies from the accepted API40 expression-decimal Salesforce packet.
func TestExpressionDecimalReceiverAPI40(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"divisionReceiver", `Decimal balance=10.01;Integer occurrences=3;System.assertEquals(3.33,(balance/occurrences).setScale(2,System.RoundingMode.DOWN));`},
		{"negativeDivisionReceiver", `Decimal balance=-10.01;Integer occurrences=3;System.assertEquals(-3.33,(balance/occurrences).setScale(2,System.RoundingMode.DOWN));`},
		{"nullReceiver", `Decimal balance;Integer occurrences=3;try{Decimal value=(balance/occurrences).setScale(2,System.RoundingMode.DOWN);System.assert(false,'Expected null receiver exception; actual '+value);}catch(NullPointerException e){System.assertNotEquals(null,e.getMessage());}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: "40.0"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
