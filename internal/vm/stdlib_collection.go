package vm

import (
	"fmt"
	"strings"
)

func callListStdlibMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "isEmpty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("List.isEmpty expects 0 arguments")
		}
		return Bool(len(receiver.List) == 0), receiver, false, true, nil
	case "clear":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("List.clear expects 0 arguments")
		}
		receiver.List = nil
		return Null, receiver, true, true, nil
	case "iterator":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("List.iterator expects 0 arguments")
		}
		return collectionIterator(receiver), receiver, false, true, nil
	case "getSObjectType":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("List.getSObjectType expects 0 arguments")
		}
		if objectName := listSObjectTypeName(receiver); objectName != "" {
			return sObjectTypeToken(objectName), receiver, false, true, nil
		}
		if elementType, ok := collectionElementType(receiver.Type); ok && strings.EqualFold(elementType, "sObject") && len(receiver.List) == 0 {
			return Null, receiver, false, true, nil
		}
		return Null, receiver, false, true, unsupportedCallError("List.getSObjectType for non-SObject list")
	default:
		return Null, receiver, false, false, nil
	}
}
func callSetStdlibMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "isEmpty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Set.isEmpty expects 0 arguments")
		}
		return Bool(len(receiver.Set) == 0), receiver, false, true, nil
	case "clear":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Set.clear expects 0 arguments")
		}
		receiver.Set = nil
		return Null, receiver, true, true, nil
	case "iterator":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Set.iterator expects 0 arguments")
		}
		return collectionIterator(receiver), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
func callMapStdlibMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "isEmpty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Map.isEmpty expects 0 arguments")
		}
		return Bool(len(receiver.Map) == 0), receiver, false, true, nil
	case "clear":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Map.clear expects 0 arguments")
		}
		clear(receiver.Map)
		clear(receiver.MapKeys)
		receiver.MapOrder = nil
		return Null, receiver, true, true, nil
	case "remove":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Map.remove expects 1 argument")
		}
		key := mapKey(args[0])
		if foldedKey, ok := caseInsensitiveStringMapStoredKey(receiver, args[0]); ok {
			key = foldedKey
		}
		value, ok := receiver.Map[key]
		if ok {
			delete(receiver.Map, key)
			delete(receiver.MapKeys, key)
			if len(receiver.MapOrder) > 0 {
				filtered := make([]string, 0, len(receiver.MapOrder))
				for _, orderedKey := range receiver.MapOrder {
					if orderedKey != key {
						filtered = append(filtered, orderedKey)
					}
				}
				receiver.MapOrder = filtered
			}
			return value, receiver, true, true, nil
		}
		return Null, receiver, false, true, nil
	case "get":
		if !caseInsensitiveStringMap(receiver) {
			return Null, receiver, false, false, nil
		}
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Map.get expects 1 argument")
		}
		key := mapKey(args[0])
		value, ok := receiver.Map[key]
		if !ok {
			if foldedKey, found := caseInsensitiveStringMapStoredKey(receiver, args[0]); found {
				value, ok = receiver.Map[foldedKey]
			}
		}
		if !ok || value.Kind == ValueNull && value.Type == "" {
			return missingMapValue(receiver), receiver, false, true, nil
		}
		return value, receiver, false, true, nil
	case "containsKey":
		if !caseInsensitiveStringMap(receiver) {
			return Null, receiver, false, false, nil
		}
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Map.containsKey expects 1 argument")
		}
		key := mapKey(args[0])
		_, ok := receiver.Map[key]
		if !ok {
			if foldedKey, found := caseInsensitiveStringMapStoredKey(receiver, args[0]); found {
				_, ok = receiver.Map[foldedKey]
			}
		}
		return Bool(ok), receiver, false, true, nil
	case "containsValue":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Map.containsValue expects 1 argument")
		}
		for _, value := range receiver.Map {
			if value.Equal(args[0]) {
				return Bool(true), receiver, false, true, nil
			}
		}
		return Bool(false), receiver, false, true, nil
	case "getSObjectType":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Map.getSObjectType expects 0 arguments")
		}
		if objectName := mapSObjectTypeName(receiver); objectName != "" {
			return sObjectTypeToken(objectName), receiver, false, true, nil
		}
		return Null, receiver, false, true, newExceptionError("System.TypeException", "Map.getSObjectType requires a concrete SObject value type")
	default:
		return Null, receiver, false, false, nil
	}
}
