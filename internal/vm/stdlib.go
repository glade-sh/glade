package vm

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

func callStdlibMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Kind {
	case ValueString:
		value, handled, err := callStringMember(receiver, method, args)
		return value, receiver, false, handled, err
	case ValueList:
		return callListStdlibMember(receiver, method, args)
	case ValueSet:
		return callSetStdlibMember(receiver, method, args)
	case ValueMap:
		return callMapStdlibMember(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callStringMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch method {
	case "length":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.length expects 0 arguments")
		}
		return Int(int64(utf8.RuneCountInString(receiver.Text))), true, nil
	case "contains":
		needle, err := stringArg("String.contains", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.Contains(receiver.Text, needle)), true, nil
	case "startsWith":
		prefix, err := stringArg("String.startsWith", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasPrefix(receiver.Text, prefix)), true, nil
	case "endsWith":
		suffix, err := stringArg("String.endsWith", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasSuffix(receiver.Text, suffix)), true, nil
	case "toLowerCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.toLowerCase expects 0 arguments")
		}
		return String(strings.ToLower(receiver.Text)), true, nil
	case "toUpperCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.toUpperCase expects 0 arguments")
		}
		return String(strings.ToUpper(receiver.Text)), true, nil
	case "substring":
		return substring(receiver.Text, args)
	default:
		return Null, false, nil
	}
}

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
		for key := range receiver.Map {
			delete(receiver.Map, key)
		}
		return Null, receiver, true, true, nil
	case "remove":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Map.remove expects 1 argument")
		}
		key := mapKey(args[0])
		value, ok := receiver.Map[key]
		if ok {
			delete(receiver.Map, key)
			return value, receiver, true, true, nil
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func stringArg(name string, args []Value) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s expects 1 argument", name)
	}
	if args[0].Kind != ValueString {
		return "", fmt.Errorf("%s expects String argument", name)
	}
	return args[0].Text, nil
}

func substring(text string, args []Value) (Value, bool, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, true, fmt.Errorf("String.substring expects 1 or 2 arguments")
	}
	if args[0].Kind != ValueInt || (len(args) == 2 && args[1].Kind != ValueInt) {
		return Null, true, fmt.Errorf("String.substring expects integer indexes")
	}
	runes := []rune(text)
	start := int(args[0].Int)
	end := len(runes)
	if len(args) == 2 {
		end = int(args[1].Int)
	}
	if start < 0 || start > len(runes) {
		return Null, true, fmt.Errorf("String substring index out of bounds: %d", start)
	}
	if end < 0 || end > len(runes) {
		return Null, true, fmt.Errorf("String substring index out of bounds: %d", end)
	}
	if start > end {
		return Null, true, fmt.Errorf("String substring start index exceeds end index")
	}
	return String(string(runes[start:end])), true, nil
}
