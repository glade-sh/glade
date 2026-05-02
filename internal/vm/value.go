package vm

import (
	"fmt"
	"strconv"
)

type Value struct {
	Kind ValueKind        `json:"kind"`
	Int  int64            `json:"int,omitempty"`
	Bool bool             `json:"bool,omitempty"`
	Text string           `json:"text,omitempty"`
	List []Value          `json:"list,omitempty"`
	Set  []Value          `json:"set,omitempty"`
	Map  map[string]Value `json:"map,omitempty"`
}

type ValueKind string

const (
	ValueNull   ValueKind = "null"
	ValueInt    ValueKind = "integer"
	ValueBool   ValueKind = "boolean"
	ValueString ValueKind = "string"
	ValueList   ValueKind = "list"
	ValueSet    ValueKind = "set"
	ValueMap    ValueKind = "map"
)

var Null = Value{Kind: ValueNull}

func Int(v int64) Value {
	return Value{Kind: ValueInt, Int: v}
}

func Bool(v bool) Value {
	return Value{Kind: ValueBool, Bool: v}
}

func String(v string) Value {
	return Value{Kind: ValueString, Text: v}
}

func List(values ...Value) Value {
	return Value{Kind: ValueList, List: values}
}

func Set(values ...Value) Value {
	out := Value{Kind: ValueSet}
	for _, value := range values {
		if !containsValue(out.Set, value) {
			out.Set = append(out.Set, value)
		}
	}
	return out
}

func Map() Value {
	return Value{Kind: ValueMap, Map: make(map[string]Value)}
}

func (v Value) String() string {
	switch v.Kind {
	case ValueNull:
		return "null"
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case ValueString:
		return v.Text
	case ValueList:
		return "List" + valuesString(v.List)
	case ValueSet:
		return "Set" + valuesString(v.Set)
	case ValueMap:
		return fmt.Sprintf("Map(size=%d)", len(v.Map))
	default:
		return fmt.Sprintf("<%s>", v.Kind)
	}
}

func (v Value) Equal(other Value) bool {
	if v.Kind != other.Kind {
		return false
	}
	switch v.Kind {
	case ValueNull:
		return true
	case ValueInt:
		return v.Int == other.Int
	case ValueBool:
		return v.Bool == other.Bool
	case ValueString:
		return v.Text == other.Text
	case ValueList:
		if len(v.List) != len(other.List) {
			return false
		}
		for i := range v.List {
			if !v.List[i].Equal(other.List[i]) {
				return false
			}
		}
		return true
	case ValueSet:
		if len(v.Set) != len(other.Set) {
			return false
		}
		for _, value := range v.Set {
			if !containsValue(other.Set, value) {
				return false
			}
		}
		return true
	case ValueMap:
		if len(v.Map) != len(other.Map) {
			return false
		}
		for key, value := range v.Map {
			if !value.Equal(other.Map[key]) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func mapKey(v Value) string {
	return string(v.Kind) + ":" + v.String()
}

func containsValue(values []Value, needle Value) bool {
	for _, value := range values {
		if value.Equal(needle) {
			return true
		}
	}
	return false
}

func valuesString(values []Value) string {
	out := "["
	for i, value := range values {
		if i > 0 {
			out += ", "
		}
		out += value.String()
	}
	return out + "]"
}
