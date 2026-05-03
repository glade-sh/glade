package vm

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

func callStdlibMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Kind {
	case ValueString:
		value, handled, err := callStringMember(receiver, method, args)
		return value, receiver, false, handled, err
	case ValueDecimal:
		return callDecimalMember(receiver, method, args)
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

func callDecimalMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "setScale":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Decimal.setScale expects Integer")
		}
		scale := int(args[0].Int)
		if scale < 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.setScale expects non-negative scale")
		}
		factor := math.Pow10(scale)
		return Decimal(math.Round(receiver.Decimal*factor) / factor), receiver, false, true, nil
	case "round":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.round expects 0 arguments")
		}
		return Int(int64(math.Round(receiver.Decimal))), receiver, false, true, nil
	case "intValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.intValue expects 0 arguments")
		}
		return Int(int64(receiver.Decimal)), receiver, false, true, nil
	case "doubleValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.doubleValue expects 0 arguments")
		}
		return receiver, receiver, false, true, nil
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
	case "trim":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.trim expects 0 arguments")
		}
		return String(strings.TrimSpace(receiver.Text)), true, nil
	case "indexOf":
		needle, err := stringArg("String.indexOf", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(strings.Index(receiver.Text, needle))), true, nil
	case "lastIndexOf":
		needle, err := stringArg("String.lastIndexOf", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(strings.LastIndex(receiver.Text, needle))), true, nil
	case "replace":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("String.replace expects target and replacement Strings")
		}
		return String(strings.ReplaceAll(receiver.Text, args[0].Text, args[1].Text)), true, nil
	case "split":
		separator, err := stringArg("String.split", args)
		if err != nil {
			return Null, true, err
		}
		parts := strings.Split(receiver.Text, separator)
		out := make([]Value, 0, len(parts))
		for _, part := range parts {
			out = append(out, String(part))
		}
		return List(out...), true, nil
	case "equalsIgnoreCase":
		other, err := stringArg("String.equalsIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.EqualFold(receiver.Text, other)), true, nil
	case "substring":
		return substring(receiver.Text, args)
	default:
		return Null, false, nil
	}
}

func stringStatic(callee string, args []Value) (Value, error) {
	switch callee {
	case "String.isBlank", "String.isNotBlank":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s expects 1 argument", callee)
		}
		blank := args[0].Kind == ValueNull || (args[0].Kind == ValueString && strings.TrimSpace(args[0].Text) == "")
		if callee == "String.isNotBlank" {
			return Bool(!blank), nil
		}
		return Bool(blank), nil
	case "String.valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("String.valueOf expects 1 argument")
		}
		return String(args[0].String()), nil
	case "String.join":
		if len(args) != 2 || args[0].Kind != ValueList || args[1].Kind != ValueString {
			return Null, fmt.Errorf("String.join expects List and separator String")
		}
		parts := make([]string, 0, len(args[0].List))
		for _, item := range args[0].List {
			parts = append(parts, item.String())
		}
		return String(strings.Join(parts, args[1].Text)), nil
	default:
		return Null, fmt.Errorf("unsupported call %q", callee)
	}
}

func patternCompile(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("Pattern.compile expects regex String")
	}
	if _, err := regexp.Compile(args[0].Text); err != nil {
		return Null, err
	}
	pattern := Object("Pattern")
	pattern.Fields["source"] = args[0]
	return pattern, nil
}

func patternMatches(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, fmt.Errorf("Pattern.matches expects regex and input Strings")
	}
	matched, err := regexp.MatchString("^(?:"+args[0].Text+")$", args[1].Text)
	if err != nil {
		return Null, err
	}
	return Bool(matched), nil
}

func callPatternMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if method != "matcher" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, receiver, false, true, fmt.Errorf("Pattern.matcher expects input String")
	}
	matcher := Object("Matcher")
	matcher.Fields["source"] = receiver.Fields["source"]
	matcher.Fields["input"] = args[0]
	matcher.Fields["index"] = Int(0)
	matcher.Fields["group"] = Null
	return matcher, receiver, false, true, nil
}

func callMatcherMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("Matcher.%s expects 0 arguments", method)
	}
	source, input, err := matcherSourceInput(receiver)
	if err != nil {
		return Null, receiver, false, true, err
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return Null, receiver, false, true, err
	}
	switch method {
	case "matches":
		return Bool(re.MatchString(input) && re.FindString(input) == input), receiver, false, true, nil
	case "find":
		start := 0
		if index, ok := receiver.Fields["index"]; ok && index.Kind == ValueInt {
			start = int(index.Int)
		}
		if start < 0 {
			start = 0
		}
		if start > len(input) {
			start = len(input)
		}
		loc := re.FindStringIndex(input[start:])
		if loc == nil {
			receiver.Fields["group"] = Null
			receiver.Fields["index"] = Int(int64(len(input) + 1))
			return Bool(false), receiver, true, true, nil
		}
		matchStart := start + loc[0]
		matchEnd := start + loc[1]
		receiver.Fields["group"] = String(input[matchStart:matchEnd])
		receiver.Fields["index"] = Int(int64(matchEnd))
		return Bool(true), receiver, true, true, nil
	case "group":
		group, ok := receiver.Fields["group"]
		if !ok || group.Kind == ValueNull {
			return Null, receiver, false, true, fmt.Errorf("Matcher.group called before a successful find")
		}
		return group, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func matcherSourceInput(receiver Value) (string, string, error) {
	source, ok := receiver.Fields["source"]
	if !ok || source.Kind != ValueString {
		return "", "", fmt.Errorf("Matcher missing Pattern source")
	}
	input, ok := receiver.Fields["input"]
	if !ok || input.Kind != ValueString {
		return "", "", fmt.Errorf("Matcher missing input")
	}
	return source.Text, input.Text, nil
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
