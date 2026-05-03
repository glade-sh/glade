package vm

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

func callStdlibMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Kind {
	case ValueInt:
		return callIntegerMember(receiver, method, args)
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

func callIntegerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "format":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Integer.format expects 0 arguments")
		}
		return String(strconv.FormatInt(receiver.Int, 10)), receiver, false, true, nil
	case "intValue", "longValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Integer.%s expects 0 arguments", method)
		}
		return receiver, receiver, false, true, nil
	case "doubleValue", "decimalValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Integer.%s expects 0 arguments", method)
		}
		return Decimal(float64(receiver.Int)), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callDecimalMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "abs":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.abs expects 0 arguments")
		}
		return Decimal(math.Abs(receiver.Decimal)), receiver, false, true, nil
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
		rounded, err := int64FromFloat("Decimal.round", math.Round(receiver.Decimal))
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(rounded), receiver, false, true, nil
	case "intValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.intValue expects 0 arguments")
		}
		converted, err := int64FromFloat("Decimal.intValue", receiver.Decimal)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(converted), receiver, false, true, nil
	case "longValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.longValue expects 0 arguments")
		}
		converted, err := int64FromFloat("Decimal.longValue", receiver.Decimal)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(converted), receiver, false, true, nil
	case "doubleValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.doubleValue expects 0 arguments")
		}
		return receiver, receiver, false, true, nil
	case "pow":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Decimal.pow expects Integer")
		}
		return Decimal(math.Pow(receiver.Decimal, float64(args[0].Int))), receiver, false, true, nil
	case "format":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.format expects 0 arguments")
		}
		return String(strconv.FormatFloat(receiver.Decimal, 'f', -1, 64)), receiver, false, true, nil
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
	case "containsIgnoreCase":
		needle, err := stringArg("String.containsIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.Contains(strings.ToLower(receiver.Text), strings.ToLower(needle))), true, nil
	case "containsAny":
		chars, err := stringArg("String.containsAny", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(stringContainsAny(receiver.Text, chars)), true, nil
	case "containsOnly":
		chars, err := stringArg("String.containsOnly", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(stringContainsOnly(receiver.Text, chars)), true, nil
	case "containsNone":
		chars, err := stringArg("String.containsNone", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(!stringContainsAny(receiver.Text, chars)), true, nil
	case "containsWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.containsWhitespace expects 0 arguments")
		}
		return Bool(strings.IndexFunc(receiver.Text, unicode.IsSpace) >= 0), true, nil
	case "countMatches":
		needle, err := stringArg("String.countMatches", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(countStringMatches(receiver.Text, needle))), true, nil
	case "startsWith":
		prefix, err := stringArg("String.startsWith", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasPrefix(receiver.Text, prefix)), true, nil
	case "startsWithIgnoreCase":
		prefix, err := stringArg("String.startsWithIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasPrefix(strings.ToLower(receiver.Text), strings.ToLower(prefix))), true, nil
	case "endsWith":
		suffix, err := stringArg("String.endsWith", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasSuffix(receiver.Text, suffix)), true, nil
	case "endsWithIgnoreCase":
		suffix, err := stringArg("String.endsWithIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.HasSuffix(strings.ToLower(receiver.Text), strings.ToLower(suffix))), true, nil
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
	case "capitalize":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.capitalize expects 0 arguments")
		}
		return String(transformFirstRune(receiver.Text, strings.ToUpper)), true, nil
	case "uncapitalize":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.uncapitalize expects 0 arguments")
		}
		return String(transformFirstRune(receiver.Text, strings.ToLower)), true, nil
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
	case "replaceAll":
		replaced, err := stringRegexReplace("String.replaceAll", receiver.Text, args, true)
		if err != nil {
			return Null, true, err
		}
		return String(replaced), true, nil
	case "replaceFirst":
		replaced, err := stringRegexReplace("String.replaceFirst", receiver.Text, args, false)
		if err != nil {
			return Null, true, err
		}
		return String(replaced), true, nil
	case "remove":
		needle, err := stringArg("String.remove", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.ReplaceAll(receiver.Text, needle, "")), true, nil
	case "removeStart":
		prefix, err := stringArg("String.removeStart", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.TrimPrefix(receiver.Text, prefix)), true, nil
	case "removeStartIgnoreCase":
		prefix, err := stringArg("String.removeStartIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		if strings.HasPrefix(strings.ToLower(receiver.Text), strings.ToLower(prefix)) {
			return String(dropFirstRunes(receiver.Text, len([]rune(prefix)))), true, nil
		}
		return receiver, true, nil
	case "removeEnd":
		suffix, err := stringArg("String.removeEnd", args)
		if err != nil {
			return Null, true, err
		}
		return String(strings.TrimSuffix(receiver.Text, suffix)), true, nil
	case "removeEndIgnoreCase":
		suffix, err := stringArg("String.removeEndIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		if strings.HasSuffix(strings.ToLower(receiver.Text), strings.ToLower(suffix)) {
			return String(dropLastRunes(receiver.Text, len([]rune(suffix)))), true, nil
		}
		return receiver, true, nil
	case "split":
		parts, err := stringRegexSplit(receiver.Text, args)
		if err != nil {
			return Null, true, err
		}
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
	case "equals":
		other, err := stringArg("String.equals", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(receiver.Text == other), true, nil
	case "compareTo":
		other, err := stringArg("String.compareTo", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(strings.Compare(receiver.Text, other))), true, nil
	case "substring":
		return substring(receiver.Text, args)
	case "left":
		length, err := stringIntArg("String.left", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if length < 0 {
			return Null, true, fmt.Errorf("String.left expects non-negative length")
		}
		if length > len(runes) {
			length = len(runes)
		}
		return String(string(runes[:length])), true, nil
	case "right":
		length, err := stringIntArg("String.right", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if length < 0 {
			return Null, true, fmt.Errorf("String.right expects non-negative length")
		}
		if length > len(runes) {
			length = len(runes)
		}
		return String(string(runes[len(runes)-length:])), true, nil
	case "leftPad":
		return stringPad(receiver.Text, args, true)
	case "rightPad":
		return stringPad(receiver.Text, args, false)
	case "center":
		return stringCenter(receiver.Text, args)
	case "mid":
		start, length, err := stringTwoIntArgs("String.mid", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if start < 0 {
			start = 0
		}
		if start > len(runes) || length <= 0 {
			return String(""), true, nil
		}
		end := start + length
		if end > len(runes) {
			end = len(runes)
		}
		return String(string(runes[start:end])), true, nil
	case "reverse":
		runes := []rune(receiver.Text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return String(string(runes)), true, nil
	case "substringAfter":
		separator, err := stringArg("String.substringAfter", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.Index(receiver.Text, separator)
		if i < 0 {
			return String(""), true, nil
		}
		return String(receiver.Text[i+len(separator):]), true, nil
	case "substringAfterLast":
		separator, err := stringArg("String.substringAfterLast", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.LastIndex(receiver.Text, separator)
		if i < 0 {
			return String(""), true, nil
		}
		return String(receiver.Text[i+len(separator):]), true, nil
	case "substringBefore":
		separator, err := stringArg("String.substringBefore", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.Index(receiver.Text, separator)
		if i < 0 {
			return receiver, true, nil
		}
		return String(receiver.Text[:i]), true, nil
	case "substringBeforeLast":
		separator, err := stringArg("String.substringBeforeLast", args)
		if err != nil {
			return Null, true, err
		}
		i := strings.LastIndex(receiver.Text, separator)
		if i < 0 {
			return receiver, true, nil
		}
		return String(receiver.Text[:i]), true, nil
	case "deleteWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.deleteWhitespace expects 0 arguments")
		}
		return String(strings.Join(strings.Fields(receiver.Text), "")), true, nil
	case "normalizeSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.normalizeSpace expects 0 arguments")
		}
		return String(strings.Join(strings.Fields(receiver.Text), " ")), true, nil
	case "isWhitespace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isWhitespace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsSpace, true)), true, nil
	case "isAlpha":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlpha expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsLetter, false)), true, nil
	case "isAlphaSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlphaSpace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsLetter(r) || r == ' ' }, true)), true, nil
	case "isAlphanumeric":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlphanumeric expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }, false)), true, nil
	case "isAlphanumericSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAlphanumericSpace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' }, true)), true, nil
	case "isNumeric":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isNumeric expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, unicode.IsDigit, false)), true, nil
	case "isNumericSpace":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isNumericSpace expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return unicode.IsDigit(r) || r == ' ' }, true)), true, nil
	case "isAllLowerCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAllLowerCase expects 0 arguments")
		}
		return Bool(stringAllLetters(receiver.Text, unicode.IsLower)), true, nil
	case "isAllUpperCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAllUpperCase expects 0 arguments")
		}
		return Bool(stringAllLetters(receiver.Text, unicode.IsUpper)), true, nil
	case "repeat":
		if len(args) == 1 && args[0].Kind == ValueInt {
			if args[0].Int < 0 {
				return Null, true, fmt.Errorf("String.repeat expects non-negative count")
			}
			return String(strings.Repeat(receiver.Text, int(args[0].Int))), true, nil
		}
		if len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueInt {
			if args[1].Int < 0 {
				return Null, true, fmt.Errorf("String.repeat expects non-negative count")
			}
			parts := make([]string, int(args[1].Int))
			for i := range parts {
				parts[i] = receiver.Text
			}
			return String(strings.Join(parts, args[0].Text)), true, nil
		}
		return Null, true, fmt.Errorf("String.repeat expects count or separator and count")
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
	case "String.isEmpty", "String.isNotEmpty":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s expects 1 argument", callee)
		}
		empty := args[0].Kind == ValueNull || (args[0].Kind == ValueString && args[0].Text == "")
		if callee == "String.isNotEmpty" {
			return Bool(!empty), nil
		}
		return Bool(empty), nil
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
		return Null, unsupportedCallError(callee)
	}
}

func numericStatic(callee string, args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("%s expects 1 argument", callee)
	}
	switch callee {
	case "Integer.valueOf", "Long.valueOf":
		switch args[0].Kind {
		case ValueInt:
			return args[0], nil
		case ValueDecimal:
			converted, err := int64FromFloat(callee, args[0].Decimal)
			if err != nil {
				return Null, err
			}
			return Int(converted), nil
		case ValueString:
			parsed, err := strconv.ParseInt(strings.TrimSpace(args[0].Text), 10, 64)
			if err != nil {
				return Null, fmt.Errorf("%s invalid integer %q", callee, args[0].Text)
			}
			return Int(parsed), nil
		default:
			return Null, fmt.Errorf("%s expects String or numeric argument", callee)
		}
	case "Decimal.valueOf", "Double.valueOf":
		switch args[0].Kind {
		case ValueDecimal:
			return args[0], nil
		case ValueInt:
			return Decimal(float64(args[0].Int)), nil
		case ValueString:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(args[0].Text), 64)
			if err != nil {
				return Null, fmt.Errorf("%s invalid decimal %q", callee, args[0].Text)
			}
			if math.IsInf(parsed, 0) || math.IsNaN(parsed) {
				return Null, fmt.Errorf("%s invalid finite decimal %q", callee, args[0].Text)
			}
			return Decimal(parsed), nil
		default:
			return Null, fmt.Errorf("%s expects String or numeric argument", callee)
		}
	default:
		return Null, unsupportedCallError(callee)
	}
}

func int64FromFloat(name string, value float64) (int64, error) {
	const int64MinFloat = -9223372036854775808.0
	const int64MaxExclusiveFloat = 9223372036854775808.0
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("%s value must be finite", name)
	}
	if value < int64MinFloat || value >= int64MaxExclusiveFloat {
		return 0, fmt.Errorf("%s value out of Integer range", name)
	}
	return int64(value), nil
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

func stringIntArg(name string, args []Value) (int, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return 0, fmt.Errorf("%s expects Integer argument", name)
	}
	return int(args[0].Int), nil
}

func stringTwoIntArgs(name string, args []Value) (int, int, error) {
	if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
		return 0, 0, fmt.Errorf("%s expects Integer arguments", name)
	}
	return int(args[0].Int), int(args[1].Int), nil
}

func dropFirstRunes(text string, count int) string {
	if count <= 0 {
		return text
	}
	runes := []rune(text)
	if count >= len(runes) {
		return ""
	}
	return string(runes[count:])
}

func dropLastRunes(text string, count int) string {
	if count <= 0 {
		return text
	}
	runes := []rune(text)
	if count >= len(runes) {
		return ""
	}
	return string(runes[:len(runes)-count])
}

func stringContainsAny(text, chars string) bool {
	for _, r := range text {
		if strings.ContainsRune(chars, r) {
			return true
		}
	}
	return false
}

func stringContainsOnly(text, chars string) bool {
	for _, r := range text {
		if !strings.ContainsRune(chars, r) {
			return false
		}
	}
	return true
}

func countStringMatches(text, needle string) int {
	if needle == "" {
		return 0
	}
	count := 0
	for start := 0; ; {
		i := strings.Index(text[start:], needle)
		if i < 0 {
			return count
		}
		count++
		start += i + len(needle)
	}
}

func stringAllRunes(text string, pred func(rune) bool, emptyValue bool) bool {
	if text == "" {
		return emptyValue
	}
	for _, r := range text {
		if !pred(r) {
			return false
		}
	}
	return true
}

func stringAllLetters(text string, pred func(rune) bool) bool {
	hasLetter := false
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		hasLetter = true
		if !pred(r) {
			return false
		}
	}
	return hasLetter
}

func stringRegexReplace(name, text string, args []Value, all bool) (string, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return "", fmt.Errorf("%s expects regex and replacement Strings", name)
	}
	re, err := regexp.Compile(args[0].Text)
	if err != nil {
		return "", err
	}
	if all {
		return re.ReplaceAllString(text, args[1].Text), nil
	}
	loc := re.FindStringIndex(text)
	if loc == nil {
		return text, nil
	}
	return text[:loc[0]] + re.ReplaceAllString(text[loc[0]:loc[1]], args[1].Text) + text[loc[1]:], nil
}

func stringRegexSplit(text string, args []Value) ([]string, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("String.split expects regex String and optional Integer limit")
	}
	if args[0].Kind != ValueString || (len(args) == 2 && args[1].Kind != ValueInt) {
		return nil, fmt.Errorf("String.split expects regex String and optional Integer limit")
	}
	re, err := regexp.Compile(args[0].Text)
	if err != nil {
		return nil, err
	}
	limit := int64(0)
	if len(args) == 2 {
		limit = args[1].Int
	}
	if limit > 0 {
		maxParts := int64(len(text) + 1)
		if limit > maxParts {
			limit = maxParts
		}
		return re.Split(text, int(limit)), nil
	}
	parts := re.Split(text, -1)
	if limit == 0 {
		for len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}
	return parts, nil
}

func transformFirstRune(text string, transform func(string) string) string {
	if text == "" {
		return text
	}
	runes := []rune(text)
	first := transform(string(runes[0]))
	return first + string(runes[1:])
}

func stringPad(text string, args []Value, left bool) (Value, bool, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, true, fmt.Errorf("String pad expects length and optional pad String")
	}
	if args[0].Kind != ValueInt {
		return Null, true, fmt.Errorf("String pad expects Integer length")
	}
	pad := " "
	if len(args) == 2 {
		if args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("String pad expects String pad")
		}
		if args[1].Text != "" {
			pad = args[1].Text
		}
	}
	width := int(args[0].Int)
	current := len([]rune(text))
	if width <= current {
		return String(text), true, nil
	}
	padding := repeatRunes(pad, width-current)
	if left {
		return String(padding + text), true, nil
	}
	return String(text + padding), true, nil
}

func stringCenter(text string, args []Value) (Value, bool, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, true, fmt.Errorf("String.center expects size and optional pad String")
	}
	if args[0].Kind != ValueInt {
		return Null, true, fmt.Errorf("String.center expects Integer size")
	}
	pad := " "
	if len(args) == 2 {
		if args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("String.center expects String pad")
		}
		if args[1].Text != "" {
			pad = args[1].Text
		}
	}
	width := int(args[0].Int)
	current := len([]rune(text))
	if width <= current {
		return String(text), true, nil
	}
	total := width - current
	left := total / 2
	right := total - left
	return String(repeatRunes(pad, left) + text + repeatRunes(pad, right)), true, nil
}

func repeatRunes(pattern string, count int) string {
	if count <= 0 {
		return ""
	}
	runes := []rune(pattern)
	if len(runes) == 0 {
		runes = []rune(" ")
	}
	var out []rune
	for len(out) < count {
		remaining := count - len(out)
		if remaining >= len(runes) {
			out = append(out, runes...)
			continue
		}
		out = append(out, runes[:remaining]...)
	}
	return string(out)
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

func callObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	switch method {
	case "toString":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Object.toString expects 0 arguments")
		}
		return String(receiver.String()), true, nil
	case "equals":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Object.equals expects 1 argument")
		}
		return Bool(receiver.Equal(args[0])), true, nil
	case "hashCode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Object.hashCode expects 0 arguments")
		}
		return Int(int64(valueHashCode(receiver))), true, nil
	default:
		return Null, false, nil
	}
}

func javaStringHashCode(text string) int32 {
	var hash int32
	for _, unit := range utf16.Encode([]rune(text)) {
		hash = 31*hash + int32(unit)
	}
	return hash
}

func valueHashCode(value Value) int32 {
	switch value.Kind {
	case ValueNull:
		return 0
	case ValueInt:
		return int32(value.Int ^ (value.Int >> 32))
	case ValueDecimal:
		return javaStringHashCode(strconv.FormatFloat(value.Decimal, 'f', -1, 64))
	case ValueBool:
		if value.Bool {
			return 1231
		}
		return 1237
	case ValueString:
		return javaStringHashCode(value.Text)
	case ValueList:
		hash := int32(1)
		for _, item := range value.List {
			hash = 31*hash + valueHashCode(item)
		}
		return hash
	case ValueSet:
		parts := make([]int, 0, len(value.Set))
		for _, item := range value.Set {
			parts = append(parts, int(valueHashCode(item)))
		}
		sort.Ints(parts)
		var hash int32
		for _, part := range parts {
			hash += int32(part)
		}
		return hash
	case ValueMap:
		keys := make([]string, 0, len(value.Map))
		for key := range value.Map {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var hash int32
		for _, key := range keys {
			hash += javaStringHashCode(key) ^ valueHashCode(value.Map[key])
		}
		return hash
	case ValueObject:
		if value.Type == "Type" {
			if typeName, ok := platformScalarValue(value); ok {
				return javaStringHashCode(typeName)
			}
		}
		if platformScalarObject(value.Type) {
			if text, ok := platformScalarValue(value); ok {
				return javaStringHashCode(value.Type + ":" + text)
			}
		}
		return javaStringHashCode(fmt.Sprintf("%p", value.Fields))
	default:
		return javaStringHashCode(value.String())
	}
}

func platformScalarValue(value Value) (string, bool) {
	raw, ok := value.Fields["value"]
	if !ok || raw.Kind != ValueString {
		return "", false
	}
	return raw.Text, true
}

func callIdMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Kind != ValueString {
		return Null, false, nil
	}
	switch method {
	case "to15":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.to15 expects 0 arguments")
		}
		if err := validateApexID(receiver.Text); err != nil {
			return Null, true, err
		}
		if len(receiver.Text) == 15 {
			return String(receiver.Text), true, nil
		}
		return String(receiver.Text[:15]), true, nil
	default:
		return Null, false, nil
	}
}

func validateApexID(text string) error {
	if len(text) != 15 && len(text) != 18 {
		return fmt.Errorf("System.StringException: Invalid id: %s", text)
	}
	for _, r := range text {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("System.StringException: Invalid id: %s", text)
		}
	}
	return nil
}

func idStatic(callee string, args []Value) (Value, error) {
	if callee != "Id.valueOf" {
		return Null, unsupportedCallError(callee)
	}
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Id.valueOf expects String[, Boolean]")
	}
	if args[0].Kind != ValueString {
		return Null, fmt.Errorf("Id.valueOf expects String")
	}
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Id.valueOf restoreCasing expects Boolean")
		}
		if args[1].Bool {
			restored, err := restoreApexIDCasing(args[0].Text)
			if err != nil {
				return Null, err
			}
			return String(restored), nil
		}
	}
	if err := validateApexID(args[0].Text); err != nil {
		return Null, err
	}
	return String(args[0].Text), nil
}

func restoreApexIDCasing(text string) (string, error) {
	if err := validateApexID(text); err != nil {
		return "", err
	}
	if len(text) != 18 {
		return text, nil
	}
	out := []byte(strings.ToLower(text[:15]))
	for chunk := 0; chunk < 3; chunk++ {
		mask, ok := apexIDChecksumMask(text[15+chunk])
		if !ok {
			return "", fmt.Errorf("System.StringException: Invalid id: %s", text)
		}
		for bit := 0; bit < 5; bit++ {
			idx := chunk*5 + bit
			if mask&(1<<bit) != 0 && out[idx] >= 'a' && out[idx] <= 'z' {
				out[idx] -= 'a' - 'A'
			}
		}
	}
	return string(out) + text[15:], nil
}

func apexIDChecksumMask(ch byte) (int, bool) {
	switch {
	case ch >= 'A' && ch <= 'Z':
		return int(ch - 'A'), true
	case ch >= '0' && ch <= '5':
		return int(ch-'0') + 26, true
	default:
		return 0, false
	}
}
