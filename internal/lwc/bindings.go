package lwc

import (
	"fmt"
	"html"
	"strconv"
	"strings"
)

type ValueKind int

const (
	ValueString ValueKind = iota
	ValueInt
	ValueBool
	ValueObject
	ValueArray
	ValueNull
)

type Value struct {
	Kind   ValueKind
	String string
	Int    int64
	Bool   bool
	Fields map[string]Value
	Items  []Value
}

type PropertyBag map[string]Value

func StringValue(s string) Value  { return Value{Kind: ValueString, String: s} }
func IntValue(i int64) Value      { return Value{Kind: ValueInt, Int: i} }
func BoolValue(b bool) Value      { return Value{Kind: ValueBool, Bool: b} }
func NullValue() Value            { return Value{Kind: ValueNull} }
func ObjectValue(fields map[string]Value) Value {
	return Value{Kind: ValueObject, Fields: fields}
}
func ArrayValue(items []Value) Value { return Value{Kind: ValueArray, Items: items} }

func PropertyBagFromStrings(in map[string]string) PropertyBag {
	out := make(PropertyBag, len(in))
	for k, v := range in {
		out[k] = StringValue(v)
	}
	return out
}

func ResolveBinding(expr string, bag PropertyBag) (string, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}") {
		expr = strings.TrimSpace(expr[1 : len(expr)-1])
	}
	val, ok := resolvePath(expr, bag)
	if !ok {
		return "", fmt.Errorf("unresolved binding %q", expr)
	}
	return formatValue(val), nil
}

func resolvePath(path string, bag PropertyBag) (Value, bool) {
	parts := strings.Split(path, ".")
	var cur Value
	var ok bool
	cur, ok = bag[parts[0]]
	if !ok {
		return Value{}, false
	}
	for _, part := range parts[1:] {
		if cur.Kind != ValueObject {
			return Value{}, false
		}
		cur, ok = cur.Fields[part]
		if !ok {
			return Value{}, false
		}
	}
	return cur, true
}

func formatValue(v Value) string {
	switch v.Kind {
	case ValueString:
		return html.EscapeString(v.String)
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case ValueNull:
		return ""
	default:
		return html.EscapeString(v.String)
	}
}

func Truthy(v Value) bool {
	switch v.Kind {
	case ValueBool:
		return v.Bool
	case ValueString:
		return strings.TrimSpace(v.String) != ""
	case ValueInt:
		return v.Int != 0
	case ValueNull:
		return false
	case ValueArray:
		return len(v.Items) > 0
	case ValueObject:
		return len(v.Fields) > 0
	default:
		return false
	}
}

func literalToValue(literal string) Value {
	literal = strings.TrimSpace(literal)
	if literal == "true" {
		return BoolValue(true)
	}
	if literal == "false" {
		return BoolValue(false)
	}
	if literal == "null" || literal == "undefined" {
		return NullValue()
	}
	if strings.HasPrefix(literal, "'") && strings.HasSuffix(literal, "'") && len(literal) >= 2 {
		return StringValue(literal[1 : len(literal)-1])
	}
	if strings.HasPrefix(literal, `"`) && strings.HasSuffix(literal, `"`) && len(literal) >= 2 {
		return StringValue(literal[1 : len(literal)-1])
	}
	if n, err := strconv.ParseInt(literal, 10, 64); err == nil {
		return IntValue(n)
	}
	return StringValue(literal)
}
