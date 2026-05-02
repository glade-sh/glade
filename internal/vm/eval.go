package vm

import (
	"fmt"
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
		if value.Kind != ValueInt {
			return Null, fmt.Errorf("operator - requires Integer, got %s", value.Kind)
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
		return intBinary(op, left, right, func(a, b int64) int64 { return a + b })
	case "-":
		return intBinary(op, left, right, func(a, b int64) int64 { return a - b })
	case "*":
		return intBinary(op, left, right, func(a, b int64) int64 { return a * b })
	case "/":
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
		if left.Kind != ValueInt || right.Kind != ValueInt {
			return Null, fmt.Errorf("operator %s requires Integer operands", op)
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
	return Int(fn(left.Int, right.Int)), nil
}

func ensureAssignable(typeName string, value Value) error {
	if value.Kind == ValueNull {
		return nil
	}
	switch typeName {
	case "Integer", "Long":
		if value.Kind == ValueInt {
			return nil
		}
	case "Boolean":
		if value.Kind == ValueBool {
			return nil
		}
	case "String":
		if value.Kind == ValueString {
			return nil
		}
	case "Object":
		return nil
	}
	if value.Kind == ValueObject {
		return nil
	}
	if strings.HasPrefix(typeName, "List<") && value.Kind == ValueList {
		return nil
	}
	if strings.HasPrefix(typeName, "Set<") && value.Kind == ValueSet {
		return nil
	}
	if strings.HasPrefix(typeName, "Map<") && value.Kind == ValueMap {
		return nil
	}
	return fmt.Errorf("cannot assign %s", value.Kind)
}

func constructValue(typeName string, args []Value) (Value, error) {
	switch {
	case strings.HasPrefix(typeName, "List<"):
		return List(args...), nil
	case strings.HasPrefix(typeName, "Set<"):
		return Set(args...), nil
	case strings.HasPrefix(typeName, "Map<"):
		if len(args) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept positional values")
		}
		return Map(), nil
	default:
		if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor does not accept arguments", typeName)
		}
		return Object(typeName), nil
	}
}
