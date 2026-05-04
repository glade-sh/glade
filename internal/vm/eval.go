package vm

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func parseLiteral(raw string) (Value, error) {
	switch raw {
	case "true":
		return Bool(true), nil
	case "false":
		return Bool(false), nil
	case "null":
		return Null, nil
	}
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") {
		text := strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'")
		return String(strings.ReplaceAll(text, "''", "'")), nil
	}
	if strings.Contains(raw, ".") {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Null, fmt.Errorf("invalid decimal literal %q", raw)
		}
		return Decimal(value), nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return Null, fmt.Errorf("invalid literal %q", raw)
	}
	return Int(value), nil
}

func evalUnary(op string, value Value) (Value, error) {
	switch op {
	case "!":
		if value.Kind != ValueBool {
			return Null, fmt.Errorf("operator ! requires Boolean, got %s", value.Kind)
		}
		return Bool(!value.Bool), nil
	case "-":
		if value.Kind == ValueDecimal {
			return Decimal(-value.Decimal), nil
		}
		if value.Kind != ValueInt {
			return Null, fmt.Errorf("operator - requires numeric value, got %s", value.Kind)
		}
		if value.Int == math.MinInt64 {
			return Null, fmt.Errorf("integer unary - overflow")
		}
		return Int(-value.Int), nil
	default:
		return Null, fmt.Errorf("unsupported unary operator %q", op)
	}
}

func evalBinary(op string, left, right Value) (Value, error) {
	switch op {
	case "+":
		if left.Kind == ValueString || right.Kind == ValueString {
			return String(left.String() + right.String()), nil
		}
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a + b })
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a + b })
	case "-":
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a - b })
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a - b })
	case "*":
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a * b })
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a * b })
	case "/":
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			if decimalOf(right) == 0 {
				return Null, fmt.Errorf("division by zero")
			}
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a / b })
		}
		if right.Kind == ValueInt && right.Int == 0 {
			return Null, fmt.Errorf("division by zero")
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a / b })
	case "%":
		if right.Kind == ValueInt && right.Int == 0 {
			return Null, fmt.Errorf("division by zero")
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a % b })
	case "==":
		return Bool(left.Equal(right)), nil
	case "!=":
		return Bool(!left.Equal(right)), nil
	case "<", "<=", ">", ">=":
		if !isNumeric(left) || !isNumeric(right) {
			return Null, fmt.Errorf("operator %s requires numeric operands", op)
		}
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			a, b := decimalOf(left), decimalOf(right)
			switch op {
			case "<":
				return Bool(a < b), nil
			case "<=":
				return Bool(a <= b), nil
			case ">":
				return Bool(a > b), nil
			default:
				return Bool(a >= b), nil
			}
		}
		switch op {
		case "<":
			return Bool(left.Int < right.Int), nil
		case "<=":
			return Bool(left.Int <= right.Int), nil
		case ">":
			return Bool(left.Int > right.Int), nil
		default:
			return Bool(left.Int >= right.Int), nil
		}
	case "&&", "||":
		if left.Kind != ValueBool || right.Kind != ValueBool {
			return Null, fmt.Errorf("operator %s requires Boolean operands", op)
		}
		if op == "&&" {
			return Bool(left.Bool && right.Bool), nil
		}
		return Bool(left.Bool || right.Bool), nil
	default:
		return Null, fmt.Errorf("unsupported binary operator %q", op)
	}
}

func intBinary(op string, left, right Value, fn func(int64, int64) int64) (Value, error) {
	if left.Kind != ValueInt || right.Kind != ValueInt {
		return Null, fmt.Errorf("operator %s requires Integer operands", op)
	}
	if err := checkIntBinaryOverflow(op, left.Int, right.Int); err != nil {
		return Null, err
	}
	return Int(fn(left.Int, right.Int)), nil
}

func decimalBinary(op string, left, right Value, fn func(float64, float64) float64) (Value, error) {
	if !isNumeric(left) || !isNumeric(right) {
		return Null, fmt.Errorf("operator %s requires numeric operands", op)
	}
	result := fn(decimalOf(left), decimalOf(right))
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return Null, fmt.Errorf("operator %s result must be finite", op)
	}
	return Decimal(result), nil
}

func checkIntBinaryOverflow(op string, left, right int64) error {
	switch op {
	case "+":
		if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
			return fmt.Errorf("operator + integer overflow")
		}
	case "-":
		if (right < 0 && left > math.MaxInt64+right) || (right > 0 && left < math.MinInt64+right) {
			return fmt.Errorf("operator - integer overflow")
		}
	case "*":
		if left != 0 && right != 0 {
			if left == math.MinInt64 && right == -1 || right == math.MinInt64 && left == -1 {
				return fmt.Errorf("operator * integer overflow")
			}
			product := left * right
			if product/right != left {
				return fmt.Errorf("operator * integer overflow")
			}
		}
	case "/":
		if left == math.MinInt64 && right == -1 {
			return fmt.Errorf("operator / integer overflow")
		}
	case "%":
		if left == math.MinInt64 && right == -1 {
			return fmt.Errorf("operator %% integer overflow")
		}
	}
	return nil
}

func isNumeric(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueDecimal
}

func decimalOf(value Value) float64 {
	if value.Kind == ValueDecimal {
		return value.Decimal
	}
	return float64(value.Int)
}

func coerceAssignable(typeName string, value Value) (Value, error) {
	if value.Kind == ValueNull {
		return value, nil
	}
	switch typeName {
	case "Integer", "Long":
		if value.Kind == ValueInt {
			return value, nil
		}
	case "Decimal", "Double":
		if value.Kind == ValueInt {
			return Decimal(float64(value.Int)), nil
		}
		if value.Kind == ValueDecimal {
			return value, nil
		}
	case "Boolean":
		if value.Kind == ValueBool {
			return value, nil
		}
	case "String", "Id":
		if value.Kind == ValueString {
			return value, nil
		}
	case "Object":
		return value, nil
	}
	if value.Kind == ValueObject {
		if strings.EqualFold(typeName, value.Type) {
			return value, nil
		}
		return Null, fmt.Errorf("cannot assign %s to %s", value.Type, typeName)
	}
	if strings.HasPrefix(typeName, "List<") && value.Kind == ValueList {
		return coerceCollectionValue(typeName, value)
	}
	if strings.HasPrefix(typeName, "Set<") && value.Kind == ValueSet {
		return coerceCollectionValue(typeName, value)
	}
	if strings.HasPrefix(typeName, "Map<") && value.Kind == ValueMap {
		return coerceCollectionValue(typeName, value)
	}
	return Null, fmt.Errorf("cannot assign %s to %s", value.Kind, typeName)
}

func ensureAssignable(typeName string, value Value) error {
	_, err := coerceAssignable(typeName, value)
	return err
}

func coerceCollectionValue(typeName string, value Value) (Value, error) {
	value.Type = typeName
	switch value.Kind {
	case ValueList:
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		for i, item := range value.List {
			coerced, err := coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			value.List[i] = coerced
		}
	case ValueSet:
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		out := make([]Value, 0, len(value.Set))
		for _, item := range value.Set {
			coerced, err := coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			if !containsValue(out, coerced) {
				out = append(out, coerced)
			}
		}
		value.Set = out
	}
	return value, nil
}

func constructValue(typeName string, args []Value) (Value, error) {
	switch {
	case strings.HasPrefix(typeName, "List<"):
		value := List(args...)
		return coerceCollectionValue(typeName, value)
	case strings.HasPrefix(typeName, "Set<"):
		value := Set(args...)
		return coerceCollectionValue(typeName, value)
	case strings.HasPrefix(typeName, "Map<"):
		if len(args) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept positional values")
		}
		value := Map()
		value.Type = typeName
		return value, nil
	default:
		if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor does not accept arguments", typeName)
		}
		return Object(typeName), nil
	}
}

func collectionElementType(typeName string) (string, bool) {
	args, ok := genericTypeArgs(typeName)
	if !ok || len(args) != 1 {
		return "", false
	}
	return args[0], true
}

func mapTypeArgs(typeName string) (string, string, bool) {
	args, ok := genericTypeArgs(typeName)
	if !ok || len(args) != 2 {
		return "", "", false
	}
	return args[0], args[1], true
}

func genericTypeArgs(typeName string) ([]string, bool) {
	open := strings.IndexByte(typeName, '<')
	if open < 0 || !strings.HasSuffix(typeName, ">") {
		return nil, false
	}
	inner := typeName[open+1 : len(typeName)-1]
	var args []string
	depth := 0
	start := 0
	for i, r := range inner {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[start:]))
	for _, arg := range args {
		if arg == "" {
			return nil, false
		}
	}
	return args, true
}
