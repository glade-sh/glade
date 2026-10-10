package vm

import (
	"fmt"
	"strings"
)

// Assertion equality and messages have their
// own contract. String == remains case-insensitive; typed Ids retain identity.
func (vm *VM) assertionEquals(left, right Value, result *Result) (bool, error) {
	return vm.assertionEqualsWithSeen(left, right, result, make(map[[2]uint64]bool))
}

func (vm *VM) assertionEqualsWithSeen(left, right Value, result *Result, seen map[[2]uint64]bool) (bool, error) {
	if left.Kind == right.Kind && (left.Kind == ValueList || left.Kind == ValueMap) && left.Ref != 0 && right.Ref != 0 {
		// J056-J061: cyclic collection assertion comparisons raise an
		// uncatchable limit error, even when both arguments are the same value.
		pair := [2]uint64{left.Ref, right.Ref}
		if seen[pair] {
			return false, &RuntimeError{Type: "System.LimitException", Message: "Maximum stack depth reached: 2", Stack: vm.stackFrames()}
		}
		seen[pair] = true
		defer delete(seen, pair)
	}
	if collectionStringLike(left) && collectionStringLike(right) &&
		!strings.EqualFold(valueTypeName(left), "Id") && !strings.EqualFold(valueTypeName(right), "Id") {
		leftText, err := vm.displayString(left, result)
		if err != nil {
			return false, err
		}
		rightText, err := vm.displayString(right, result)
		if err != nil {
			return false, err
		}
		return leftText == rightText, nil
	}
	if left.Kind == ValueList && right.Kind == ValueList {
		if len(left.List) != len(right.List) {
			return false, nil
		}
		for i := range left.List {
			equal, err := vm.assertionCollectionElementEquals(left.List[i], right.List[i], result, seen)
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	}
	if left.Kind == ValueMap && right.Kind == ValueMap {
		if len(left.Map) != len(right.Map) {
			return false, nil
		}
		for _, raw := range orderedValueMapKeys(left) {
			leftValue := left.Map[raw]
			key, err := vm.resolvedMapLookupKey(right, mapStoredKey(left, raw))
			if err != nil {
				return false, err
			}
			rightValue, ok := right.Map[key]
			if !ok {
				return false, nil
			}
			equal, err := vm.assertionCollectionElementEquals(leftValue, rightValue, result, seen)
			if err != nil || !equal {
				return equal, err
			}
		}
		return true, nil
	}
	return vm.apexEquals(left, right, result)
}

func (vm *VM) assertionCollectionElementEquals(left, right Value, result *Result, seen map[[2]uint64]bool) (bool, error) {
	// J039/J043-J049: collection values preserve numeric types and exact
	// String/Id text. J008/J032/J047 retain custom equals and typed Ids.
	if (left.Kind == ValueInt || left.Kind == ValueDecimal) && (right.Kind == ValueInt || right.Kind == ValueDecimal) &&
		!strings.EqualFold(runtimeValueTypeName(left), runtimeValueTypeName(right)) {
		return false, nil
	}
	if collectionStringLike(left) && collectionStringLike(right) ||
		collectionStringLike(left) && strings.EqualFold(runtimeValueTypeName(right), "Id") ||
		strings.EqualFold(runtimeValueTypeName(left), "Id") && collectionStringLike(right) {
		leftText, err := vm.displayString(left, result)
		if err != nil {
			return false, err
		}
		rightText, err := vm.displayString(right, result)
		return leftText == rightText, err
	}
	if left.Kind == ValueObject && right.Kind == ValueObject && left.equal(right, make(map[[2]uint64]bool)) {
		return true, nil
	}
	return vm.assertionEqualsWithSeen(left, right, result, seen)
}

func assertionComparisonTypeName(value Value) string {
	typeName := runtimeValueTypeName(value)
	// J036/J050/J051: core enum diagnostics retain their System namespace.
	if !strings.Contains(typeName, ".") {
		_, _, coreEnum := coreEnumSpec(typeName)
		if coreEnum || strings.EqualFold(typeName, "StatusCode") {
			return "System." + typeName
		}
	}
	return typeName
}

func (vm *VM) assertionMessage(base string, extra []Value, prefix bool, result *Result) (string, error) {
	if len(extra) == 0 || extra[0].Kind == ValueNull {
		return base, nil
	}
	message, err := vm.displayString(extra[0], result)
	if err != nil {
		return "", err
	}
	if prefix {
		return message + ": " + base, nil
	}
	if message == "" {
		return base + ":", nil
	}
	return base + ": " + message, nil
}

func (vm *VM) callAssertion(callee string, args []Value, result *Result) (Value, error) {
	modern := strings.HasPrefix(callee, "Assert.")
	switch callee {
	case "System.assert", "Assert.isTrue", "Assert.isFalse":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("%s expects 1 or 2 arguments", callee)
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
		}
		if args[0].Kind != ValueBool {
			return Null, fmt.Errorf("%s expects Boolean, got %s", callee, args[0].Kind)
		}
		if modern && len(args) == 2 && args[1].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Message parameter cannot be null")
		}
		want := callee != "Assert.isFalse"
		if args[0].Bool == want {
			return Null, nil
		}
		message, err := vm.assertionMessage("Assertion Failed", args[1:], false, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError(message)
	case "System.assertEquals", "System.assertNotEquals", "Assert.areEqual", "Assert.areNotEqual":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("%s expects 2 or 3 arguments", callee)
		}
		if len(args) == 3 && args[2].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Argument 3 cannot be null")
		}
		if args[0].Kind != ValueNull && args[1].Kind != ValueNull {
			leftType, rightType := runtimeValueTypeName(args[0]), runtimeValueTypeName(args[1])
			// J012/J019-J051: Object-typed arguments still require compatible
			// runtime types. Scalar numeric, String/Id and Date/Datetime pairs
			// remain compatible; collection element equality is stricter.
			numeric := (args[0].Kind == ValueInt || args[0].Kind == ValueDecimal) &&
				(args[1].Kind == ValueInt || args[1].Kind == ValueDecimal)
			stringID := strings.EqualFold(leftType, "String") && strings.EqualFold(rightType, "Id") ||
				strings.EqualFold(leftType, "Id") && strings.EqualFold(rightType, "String")
			dateTime := strings.EqualFold(leftType, "Date") && strings.EqualFold(rightType, "Datetime") ||
				strings.EqualFold(leftType, "Datetime") && strings.EqualFold(rightType, "Date")
			collections := args[0].Kind == args[1].Kind && (args[0].Kind == ValueList || args[0].Kind == ValueSet || args[0].Kind == ValueMap)
			if !numeric && !stringID && !dateTime && !collections && !vm.typeMatches(leftType, rightType, make(map[string]bool)) &&
				!vm.typeMatches(rightType, leftType, make(map[string]bool)) {
				return Null, newExceptionError("System.TypeException", "Comparison arguments must be compatible types: "+assertionComparisonTypeName(args[0])+", "+assertionComparisonTypeName(args[1]))
			}
		}
		equal, err := vm.assertionEquals(args[0], args[1], result)
		if err != nil {
			return Null, err
		}
		wantEqual := callee == "System.assertEquals" || callee == "Assert.areEqual"
		if equal == wantEqual {
			return Null, nil
		}
		expected, err := vm.displayString(args[0], result)
		if err != nil {
			return Null, err
		}
		base := "Same value: " + expected
		if wantEqual {
			actual, err := vm.displayString(args[1], result)
			if err != nil {
				return Null, err
			}
			base = "Expected: " + expected + ", Actual: " + actual
		}
		message, err := vm.assertionMessage(base, args[2:], true, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError("Assertion Failed: " + message)
	case "Assert.isNull", "Assert.isNotNull":
		if len(args) != 1 && len(args) != 2 {
			return Null, fmt.Errorf("%s expects 1 or 2 arguments", callee)
		}
		if len(args) == 2 && args[1].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Message parameter cannot be null")
		}
		wantNull := callee == "Assert.isNull"
		if (args[0].Kind == ValueNull) == wantNull {
			return Null, nil
		}
		base := "Instance expected to be a non null value"
		if wantNull {
			base = "Nullable object asserted with non null value"
		}
		message, err := vm.assertionMessage(base, args[1:], false, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError("Assertion Failed: " + message)
	case "Assert.isInstanceOfType", "Assert.isNotInstanceOfType":
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("%s expects value, Type[, message]", callee)
		}
		if args[1].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
		}
		if args[1].Kind != ValueObject || args[1].Type != "Type" {
			return Null, fmt.Errorf("%s expects Type as second argument", callee)
		}
		expectedType := typeValueName(args[1])
		actualType := valueTypeName(args[0])
		if args[0].Kind == ValueObject {
			actualType = runtimeObjectType(args[0])
		}
		identity := typeValueIdentityName(args[1])
		matches := args[0].Kind != ValueNull && vm.typeMatches(actualType, identity, make(map[string]bool))
		if args[0].Kind == ValueObject {
			if matched, handled := vm.schemaRecordValueMatches(args[0], identity); handled {
				matches = matched
			}
		}
		// T191/T192: Integer is admitted by the Long type assertion.
		if args[0].Kind == ValueInt && strings.EqualFold(expectedType, "Long") {
			matches = true
		}
		wantInstance := callee == "Assert.isInstanceOfType"
		if matches == wantInstance {
			return Null, nil
		}
		base := "The provided object is an instance of class " + expectedType
		if wantInstance {
			base = "The provided object is not an instance of class " + expectedType
		}
		message, err := vm.assertionMessage(base, args[2:], false, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError("Assertion Failed: " + message)
	case "Assert.fail":
		if len(args) > 1 {
			return Null, fmt.Errorf("Assert.fail expects 0 or 1 arguments")
		}
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Message parameter cannot be null")
		}
		message, err := vm.assertionMessage("Assertion failed", args, false, result)
		if err != nil {
			return Null, err
		}
		return Null, vm.assertError("Assertion Failed: " + message)
	}
	return Null, unsupportedCallError(callee)
}
