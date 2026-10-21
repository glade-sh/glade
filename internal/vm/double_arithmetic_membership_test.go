package vm

import "testing"

// A08 P17/P18/P20/P21/P22/P24/P25/P26/P28/P29/P30/P32 observe
// arithmetic results boxed as Object, with both Double and Decimal membership.
// The API 62/67 anonymous captures and owned API 68 export agree. This does
// not assert membership for stub callback arguments or other Double producers.
func TestExecDoubleArithmeticTypeMembershipMatchesNativeRows(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, operation := range []struct{ operator, text string }{
			{"+", "3.5"}, {"-", "-0.5"}, {"*", "3.0"}, {"/", "0.75"},
		} {
			for _, right := range []struct{ typ, literal string }{
				{"Integer", "2"}, {"Long", "2L"}, {"Double", "2.00"},
			} {
				t.Run(api+"/"+operation.operator+"/"+right.typ, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions("Double left = 1.50; "+right.typ+" right = "+right.literal+"; Object observed = left "+operation.operator+" right; String actual = String.valueOf(observed) + '|' + (observed instanceof Double) + '|' + (observed instanceof Decimal); System.assert('"+operation.text+"|true|true'.equals(actual), actual);", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := New(nil).Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
