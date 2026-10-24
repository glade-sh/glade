package vm

import (
	"fmt"
	"math"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
)

func (vm *VM) flowActionDeclaredInput(elementType, name string, value Value) (Value, error) {
	field, _, declared := vm.lookupField(elementType, name)
	if !declared || !strings.EqualFold(field.Type, "Integer") || value.Kind != ValueDecimal {
		return value, nil
	}
	number, exact := valueDecimalRat(value)
	if exact && number.IsInt() && number.Num().IsInt64() {
		integer := number.Num().Int64()
		if integer >= math.MinInt32 && integer <= math.MaxInt32 {
			return Int(integer), nil
		}
	} else if !exact && isFloatBackedDecimal(value) && !math.IsNaN(value.Decimal) && !math.IsInf(value.Decimal, 0) && math.Trunc(value.Decimal) == value.Decimal && value.Decimal >= math.MinInt32 && value.Decimal <= math.MaxInt32 {
		return Int(int64(value.Decimal)), nil
	}
	return Null, &dml.FlowActionInputError{Cause: fmt.Errorf("Flow input %s cannot convert Decimal to Integer", name)}
}
