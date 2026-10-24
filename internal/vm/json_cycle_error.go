package vm

import (
	"fmt"
	"strings"
)

// Diagnostic rendering runs only after a public JSON operation finds a cycle.
// It uses backing identity plus logical list references, rather than invoking
// user toString methods or the legacy collection display path.
func (vm *VM) jsonCircularReferenceError(value Value, state *jsonSerializationState) error {
	if value.Kind == ValueList && len(value.List) == 0 && value.Ref != 0 {
		if current, ok := jsonCycleListSnapshot(state.rootValue, value.Ref, make(map[jsonSerializationIdentity]bool)); ok {
			value = current
		}
	}
	budget := 1024
	text := vm.jsonCycleDiagnostic(value, make(map[jsonSerializationIdentity]bool), &budget)
	message := "Cycle detected. Multiple references to " + text
	if state.generator {
		switch state.rootValue.Kind {
		case ValueMap:
			message += fmt.Sprintf(" (through reference chain: java.util.LinkedHashMap[%q])", state.rootKey)
		case ValueList:
			message += fmt.Sprintf(" (through reference chain: java.util.ArrayList[%d])", state.rootIndex)
		}
	}
	return newExceptionError("System.JSONException", message)
}

func jsonCycleListSnapshot(value Value, ref uint64, seen map[jsonSerializationIdentity]bool) (Value, bool) {
	if value.Kind == ValueList && value.Ref == ref && len(value.List) > 0 {
		return value, true
	}
	if identity, ok := jsonSerializationToken(value); ok {
		if seen[identity] {
			return Null, false
		}
		seen[identity] = true
	}
	for _, child := range value.List {
		if found, ok := jsonCycleListSnapshot(child, ref, seen); ok {
			return found, true
		}
	}
	for _, child := range value.Set {
		if found, ok := jsonCycleListSnapshot(child, ref, seen); ok {
			return found, true
		}
	}
	for _, child := range value.Map {
		if found, ok := jsonCycleListSnapshot(child, ref, seen); ok {
			return found, true
		}
	}
	for _, child := range value.Fields {
		if found, ok := jsonCycleListSnapshot(child, ref, seen); ok {
			return found, true
		}
	}
	return Null, false
}

func (vm *VM) jsonCycleDiagnostic(value Value, active map[jsonSerializationIdentity]bool, budget *int) string {
	if *budget <= 0 {
		return "..."
	}
	*budget--
	identity, ok := jsonSerializationToken(value)
	if value.Kind == ValueList && value.Ref != 0 {
		identity = jsonSerializationIdentity{kind: jsonListReferenceKind, token: value.Ref}
		ok = true
	}
	if ok {
		if active[identity] {
			return "(already output)"
		}
		active[identity] = true
		defer delete(active, identity)
	}
	parts := []string{}
	open, close := "", ""
	switch value.Kind {
	case ValueList, ValueSet:
		open, close = "(", ")"
		children := value.List
		if value.Kind == ValueSet {
			open, close = "{", "}"
			children = value.Set
		}
		for _, child := range children {
			parts = append(parts, vm.jsonCycleDiagnostic(child, active, budget))
			if *budget <= 0 {
				break
			}
		}
	case ValueMap:
		open, close = "{", "}"
		for _, key := range sortedMapKeys(value.Map) {
			parts = append(parts, mapStoredKey(value, key).String()+"="+vm.jsonCycleDiagnostic(value.Map[key], active, budget))
			if *budget <= 0 {
				break
			}
		}
	case ValueObject:
		if scalar, ok := jsonPlatformScalarFromValue(value); ok {
			return fmt.Sprint(scalar)
		}
		name := value.Type
		if dot := strings.LastIndex(name, "."); dot >= 0 {
			name = name[dot+1:]
		}
		open, close = name+":[", "]"
		for _, key := range sortedMapKeys(value.Fields) {
			if isInheritedInstanceFieldStorage(key) || isLexicalInstanceFieldStorage(key) || isInternalSObjectField(key) {
				continue
			}
			parts = append(parts, key+"="+vm.jsonCycleDiagnostic(value.Fields[key], active, budget))
			if *budget <= 0 {
				break
			}
		}
	default:
		return value.String()
	}
	return open + strings.Join(parts, ", ") + close
}
