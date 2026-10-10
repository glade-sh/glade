package vm

import (
	"fmt"
	"testing"
)

// Typed numeric cases from the accepted API40 numeric-null packet.
func TestNullNumericArithmeticAPI40(t *testing.T) {
	for _, typeName := range []string{"Integer", "Long"} {
		for _, op := range []string{"+", "-", "*", "/"} {
			for _, expression := range []string{"missing " + op + " value", "value " + op + " missing"} {
				t.Run(typeName+"_"+expression, func(t *testing.T) {
					source := fmt.Sprintf(`%s missing;%s value=3;Boolean caught=false;try{%s result=%s;}catch(NullPointerException e){caught=true;}System.assert(caught);`, typeName, typeName, typeName, expression)
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
	}
	for _, typeName := range []string{"Decimal", "Integer", "Long"} {
		for _, op := range []string{"+", "-", "*", "/"} {
			t.Run(typeName+"_compound_"+op, func(t *testing.T) {
				source := fmt.Sprintf(`%s missing;%s value=3;Boolean caught=false;try{value%s=missing;}catch(NullPointerException e){caught=true;}System.assert(caught);System.assertEquals(3,value);`, typeName, typeName, op)
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
}
