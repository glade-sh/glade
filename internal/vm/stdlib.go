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
	case ValueObject:
		if isIteratorValue(receiver) {
			return callIteratorMember(receiver, method, args)
		}
		return Null, receiver, false, false, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callIntegerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "format":
		if len(args) != 0 {
			return Null, receiver, false, true, unsupportedCallError("numeric format locale/pattern overloads")
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
		if err := ensureFiniteDecimal("Decimal.abs", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		return Decimal(math.Abs(receiver.Decimal)), receiver, false, true, nil
	case "setScale":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.setScale expects scale and optional RoundingMode")
		}
		if args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Decimal.setScale expects Integer scale")
		}
		if args[0].Int < 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.setScale expects non-negative scale")
		}
		mode := "HALF_UP"
		if len(args) == 2 {
			parsedMode, err := decimalRoundingMode(args[1])
			if err != nil {
				return Null, receiver, false, true, err
			}
			mode = parsedMode
		}
		rounded, err := roundDecimalToScale("Decimal.setScale", receiver.Decimal, args[0].Int, mode)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Decimal(rounded), receiver, false, true, nil
	case "round":
		if len(args) > 1 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.round expects optional RoundingMode")
		}
		mode := "HALF_UP"
		if len(args) == 1 {
			parsedMode, err := decimalRoundingMode(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			mode = parsedMode
		}
		roundedDecimal, err := roundDecimalToScale("Decimal.round", receiver.Decimal, 0, mode)
		if err != nil {
			return Null, receiver, false, true, err
		}
		rounded, err := int64FromFloat("Decimal.round", roundedDecimal)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(rounded), receiver, false, true, nil
	case "intValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.intValue expects 0 arguments")
		}
		converted, err := int32FromFloat("Decimal.intValue", receiver.Decimal)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(converted)), receiver, false, true, nil
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
		if err := ensureFiniteDecimal("Decimal.doubleValue", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		return receiver, receiver, false, true, nil
	case "pow":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Decimal.pow expects Integer")
		}
		if err := ensureFiniteDecimal("Decimal.pow", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		value := math.Pow(receiver.Decimal, float64(args[0].Int))
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return Null, receiver, false, true, fmt.Errorf("Decimal.pow result must be finite")
		}
		return Decimal(value), receiver, false, true, nil
	case "format":
		if len(args) != 0 {
			return Null, receiver, false, true, unsupportedCallError("numeric format locale/pattern overloads")
		}
		if err := ensureFiniteDecimal("Decimal.format", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		return String(strconv.FormatFloat(receiver.Decimal, 'f', -1, 64)), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func decimalRoundingMode(value Value) (string, error) {
	if value.Kind != ValueObject || value.Type != "RoundingMode" {
		return "", fmt.Errorf("Decimal rounding expects RoundingMode")
	}
	switch value.Text {
	case "UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY":
		return value.Text, nil
	default:
		return "", fmt.Errorf("unsupported Decimal rounding mode %q", value.Text)
	}
}

func roundDecimalToScale(callee string, value float64, scaleValue int64, mode string) (float64, error) {
	const maxLocalScale int64 = 15
	if err := ensureFiniteDecimal(callee, value); err != nil {
		return 0, err
	}
	if scaleValue > maxLocalScale {
		return 0, fmt.Errorf("%s scale greater than %d is not supported by the local decimal model", callee, maxLocalScale)
	}
	factor := math.Pow10(int(scaleValue))
	scaled := value * factor
	if math.IsInf(scaled, 0) || math.IsNaN(scaled) {
		return 0, fmt.Errorf("%s scaled value must be finite", callee)
	}
	rounded, err := roundScaledDecimal(callee, scaled, mode)
	if err != nil {
		return 0, err
	}
	return rounded / factor, nil
}

func ensureFiniteDecimal(callee string, value float64) error {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return fmt.Errorf("%s value must be finite", callee)
	}
	return nil
}

func roundScaledDecimal(callee string, value float64, mode string) (float64, error) {
	switch mode {
	case "UP":
		if value < 0 {
			return math.Floor(value), nil
		}
		return math.Ceil(value), nil
	case "DOWN":
		return math.Trunc(value), nil
	case "CEILING":
		return math.Ceil(value), nil
	case "FLOOR":
		return math.Floor(value), nil
	case "HALF_UP":
		return math.Round(value), nil
	case "HALF_EVEN":
		return math.RoundToEven(value), nil
	case "HALF_DOWN":
		truncated := math.Trunc(value)
		fraction := math.Abs(value - truncated)
		if math.Abs(fraction-0.5) <= 1e-12 {
			return truncated, nil
		}
		return math.Round(value), nil
	case "UNNECESSARY":
		if value != math.Trunc(value) {
			return 0, fmt.Errorf("%s rounding necessary for RoundingMode.UNNECESSARY", callee)
		}
		return value, nil
	default:
		return 0, fmt.Errorf("unsupported Decimal rounding mode %q", mode)
	}
}

func roundingModeStatic(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("RoundingMode.valueOf expects String")
	}
	mode := args[0].Text
	if !isDecimalRoundingModeName(mode) {
		return Null, fmt.Errorf("unsupported Decimal rounding mode %q", mode)
	}
	return Value{Kind: ValueObject, Type: "RoundingMode", Text: mode}, nil
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
	case "indexOfAny":
		chars, err := stringArg("String.indexOfAny", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfAny(receiver.Text, chars))), true, nil
	case "indexOfAnyBut":
		chars, err := stringArg("String.indexOfAnyBut", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfAnyBut(receiver.Text, chars))), true, nil
	case "lastIndexOfAny":
		chars, err := stringArg("String.lastIndexOfAny", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOfAny(receiver.Text, chars))), true, nil
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
	case "escapeCsv":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeCsv expects 0 arguments")
		}
		return String(escapeCSV(receiver.Text)), true, nil
	case "unescapeCsv":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeCsv expects 0 arguments")
		}
		return String(unescapeCSV(receiver.Text)), true, nil
	case "escapeHtml3", "escapeHtml4":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeHTMLCore(receiver.Text)), true, nil
	case "unescapeHtml3", "unescapeHtml4":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeHTMLEntities(receiver.Text)), true, nil
	case "escapeXml", "escapeXml10", "escapeXml11":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML(receiver.Text)), true, nil
	case "unescapeXml", "unescapeXml10", "unescapeXml11":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text)), true, nil
	case "escapeJava":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeJava expects 0 arguments")
		}
		return String(escapeJavaLike(receiver.Text, false, false)), true, nil
	case "unescapeJava":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeJava expects 0 arguments")
		}
		unescaped, err := unescapeJavaLike("String.unescapeJava", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		return String(unescaped), true, nil
	case "escapeEcmaScript":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeEcmaScript expects 0 arguments")
		}
		return String(escapeJavaLike(receiver.Text, true, true)), true, nil
	case "unescapeEcmaScript":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeEcmaScript expects 0 arguments")
		}
		unescaped, err := unescapeJavaLike("String.unescapeEcmaScript", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		return String(unescaped), true, nil
	case "escapeUnicode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.escapeUnicode expects 0 arguments")
		}
		return String(escapeUnicode(receiver.Text)), true, nil
	case "unescapeUnicode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.unescapeUnicode expects 0 arguments")
		}
		unescaped, err := unescapeJavaLike("String.unescapeUnicode", receiver.Text)
		if err != nil {
			return Null, true, err
		}
		return String(unescaped), true, nil
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
	case "ordinalIndexOf":
		needle, ordinal, err := stringStringIntArgs("String.ordinalIndexOf", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringOrdinalIndexOf(receiver.Text, needle, ordinal, false))), true, nil
	case "lastOrdinalIndexOf":
		needle, ordinal, err := stringStringIntArgs("String.lastOrdinalIndexOf", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringOrdinalIndexOf(receiver.Text, needle, ordinal, true))), true, nil
	case "replace":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("String.replace expects target and replacement Strings")
		}
		if args[0].Text == "" {
			return receiver, true, nil
		}
		return String(strings.ReplaceAll(receiver.Text, args[0].Text, args[1].Text)), true, nil
	case "replaceOnce":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("String.replaceOnce expects target and replacement Strings")
		}
		return String(stringReplaceLiteral(receiver.Text, args[0].Text, args[1].Text, false, true)), true, nil
	case "replaceIgnoreCase":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("String.replaceIgnoreCase expects target and replacement Strings")
		}
		return String(stringReplaceLiteral(receiver.Text, args[0].Text, args[1].Text, true, false)), true, nil
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
	case "removeIgnoreCase":
		needle, err := stringArg("String.removeIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringReplaceLiteral(receiver.Text, needle, "", true, false)), true, nil
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
	case "charAt":
		index, err := stringIntArg("String.charAt", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if index < 0 || index >= len(runes) {
			return Null, true, fmt.Errorf("String.charAt index out of bounds: %d", index)
		}
		return String(string(runes[index])), true, nil
	case "codePointAt":
		index, err := stringIntArg("String.codePointAt", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if index < 0 || index >= len(runes) {
			return Null, true, fmt.Errorf("String.codePointAt index out of bounds: %d", index)
		}
		return Int(int64(runes[index])), true, nil
	case "codePointBefore":
		index, err := stringIntArg("String.codePointBefore", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if index <= 0 || index > len(runes) {
			return Null, true, fmt.Errorf("String.codePointBefore index out of bounds: %d", index)
		}
		return Int(int64(runes[index-1])), true, nil
	case "codePointCount":
		begin, end, err := stringTwoIntArgs("String.codePointCount", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		if begin < 0 || end < begin || end > len(runes) {
			return Null, true, fmt.Errorf("String.codePointCount index out of bounds")
		}
		return Int(int64(end - begin)), true, nil
	case "getChars", "toCharArray":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		chars := make([]Value, 0, len([]rune(receiver.Text)))
		for _, r := range receiver.Text {
			chars = append(chars, Int(int64(r)))
		}
		return List(chars...), true, nil
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
	case "overlay":
		overlay, start, end, err := stringStringTwoIntArgs("String.overlay", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringOverlay(receiver.Text, overlay, start, end)), true, nil
	case "rotate":
		shift, err := stringIntArg("String.rotate", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringRotate(receiver.Text, shift)), true, nil
	case "swapCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.swapCase expects 0 arguments")
		}
		return String(stringSwapCase(receiver.Text)), true, nil
	case "abbreviate":
		abbreviated, err := stringAbbreviate(receiver.Text, args)
		if err != nil {
			return Null, true, err
		}
		return String(abbreviated), true, nil
	case "difference":
		other, err := stringArg("String.difference", args)
		if err != nil {
			return Null, true, err
		}
		return String(stringDifference(receiver.Text, other)), true, nil
	case "commonPrefix":
		other, err := stringArg("String.commonPrefix", args)
		if err != nil {
			return Null, true, err
		}
		return String(commonPrefix([]string{receiver.Text, other})), true, nil
	case "getLevenshteinDistance":
		other, err := stringArg("String.getLevenshteinDistance", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(levenshteinDistance(receiver.Text, other))), true, nil
	case "splitByCharacterType":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.splitByCharacterType expects 0 arguments")
		}
		return stringList(splitByCharacterType(receiver.Text, false)), true, nil
	case "splitByCharacterTypeCamelCase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.splitByCharacterTypeCamelCase expects 0 arguments")
		}
		return stringList(splitByCharacterType(receiver.Text, true)), true, nil
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
	case "strip":
		stripped, err := stringStrip(receiver.Text, args, stripBoth)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "stripStart":
		stripped, err := stringStrip(receiver.Text, args, stripStart)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "stripEnd":
		stripped, err := stringStrip(receiver.Text, args, stripEnd)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
	case "stripToNull":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripToNull expects 0 arguments")
		}
		stripped, err := stringStrip(receiver.Text, args, stripBoth)
		if err != nil {
			return Null, true, err
		}
		if stripped == "" {
			return Null, true, nil
		}
		return String(stripped), true, nil
	case "stripToEmpty":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripToEmpty expects 0 arguments")
		}
		stripped, err := stringStrip(receiver.Text, args, stripBoth)
		if err != nil {
			return Null, true, err
		}
		return String(stripped), true, nil
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
	case "isAsciiPrintable":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.isAsciiPrintable expects 0 arguments")
		}
		return Bool(stringAllRunes(receiver.Text, func(r rune) bool { return r >= 32 && r < 127 }, true)), true, nil
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
	case "String.format":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueList {
			return Null, fmt.Errorf("String.format expects format String and List arguments")
		}
		return String(formatString(args[0].Text, args[1].List)), nil
	case "String.getCommonPrefix":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("String.getCommonPrefix expects List argument")
		}
		texts := make([]string, 0, len(args[0].List))
		for _, item := range args[0].List {
			if item.Kind != ValueString {
				return Null, fmt.Errorf("String.getCommonPrefix expects List<String>")
			}
			texts = append(texts, item.Text)
		}
		return String(commonPrefix(texts)), nil
	case "String.getLevenshteinDistance":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, fmt.Errorf("String.getLevenshteinDistance expects two Strings")
		}
		return Int(int64(levenshteinDistance(args[0].Text, args[1].Text))), nil
	case "String.stripAll":
		return stringStaticStripAll(args)
	case "String.fromCharArray":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, fmt.Errorf("String.fromCharArray expects List<Integer>")
		}
		var b strings.Builder
		for _, item := range args[0].List {
			if item.Kind != ValueInt || item.Int < 0 || item.Int > utf8.MaxRune {
				return Null, fmt.Errorf("String.fromCharArray expects valid code points")
			}
			b.WriteRune(rune(item.Int))
		}
		return String(b.String()), nil
	case "String.escapeSingleQuotes":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("String.escapeSingleQuotes expects String argument")
		}
		return String(strings.ReplaceAll(args[0].Text, "'", "\\'")), nil
	default:
		return Null, unsupportedCallError(callee)
	}
}

func numericStatic(callee string, args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("%s expects 1 argument", callee)
	}
	switch callee {
	case "Integer.valueOf":
		switch args[0].Kind {
		case ValueInt:
			converted, err := int32FromFloat(callee, float64(args[0].Int))
			if err != nil {
				return Null, err
			}
			return Int(int64(converted)), nil
		case ValueDecimal:
			converted, err := int32FromFloat(callee, args[0].Decimal)
			if err != nil {
				return Null, err
			}
			return Int(int64(converted)), nil
		case ValueString:
			parsed, err := strconv.ParseInt(strings.TrimSpace(args[0].Text), 10, 32)
			if err != nil {
				return Null, fmt.Errorf("%s invalid integer %q", callee, args[0].Text)
			}
			return Int(parsed), nil
		default:
			return Null, fmt.Errorf("%s expects String or numeric argument", callee)
		}
	case "Long.valueOf":
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
		return 0, fmt.Errorf("%s value out of 64-bit integer range", name)
	}
	return int64(value), nil
}

func int32FromFloat(name string, value float64) (int32, error) {
	const int32MinFloat = -2147483648.0
	const int32MaxExclusiveFloat = 2147483648.0
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("%s value must be finite", name)
	}
	if value < int32MinFloat || value >= int32MaxExclusiveFloat {
		return 0, fmt.Errorf("%s value out of Integer range", name)
	}
	return int32(value), nil
}

func patternCompile(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("Pattern.compile expects regex String")
	}
	if feature := unsupportedJavaRegexFeature(args[0].Text); feature != "" {
		return Null, unsupportedCallError("Pattern.compile " + feature)
	}
	if _, err := regexp.Compile(args[0].Text); err != nil {
		return Null, fmt.Errorf("Pattern.compile invalid regex: %w", err)
	}
	pattern := Object("Pattern")
	pattern.Fields["source"] = args[0]
	return pattern, nil
}

func patternMatches(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, fmt.Errorf("Pattern.matches expects regex and input Strings")
	}
	if feature := unsupportedJavaRegexFeature(args[0].Text); feature != "" {
		return Null, unsupportedCallError("Pattern.matches " + feature)
	}
	matched, err := regexp.MatchString("^(?:"+args[0].Text+")$", args[1].Text)
	if err != nil {
		return Null, fmt.Errorf("Pattern.matches invalid regex: %w", err)
	}
	return Bool(matched), nil
}

func callPatternMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "matcher":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern.matcher expects input String")
		}
		if _, ok := receiver.Fields["source"]; !ok {
			return Null, receiver, false, true, fmt.Errorf("Pattern missing source")
		}
		matcher := Object("Matcher")
		matcher.Fields["source"] = receiver.Fields["source"]
		matcher.Fields["input"] = args[0]
		matcherClearMatch(matcher)
		matcher.Fields["index"] = Int(0)
		matcher.Fields["regionStart"] = Int(0)
		matcher.Fields["regionEnd"] = Int(int64(utf8.RuneCountInString(args[0].Text)))
		return matcher, receiver, false, true, nil
	case "pattern":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Pattern.pattern expects 0 arguments")
		}
		source, ok := receiver.Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern missing source")
		}
		return source, receiver, false, true, nil
	case "split":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
		}
		source, ok := receiver.Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern missing source")
		}
		parts, err := patternSplit(source.Text, args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		out := make([]Value, 0, len(parts))
		for _, part := range parts {
			out = append(out, String(part))
		}
		return List(out...), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callMatcherMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	source, input, err := matcherSourceInput(receiver)
	if err != nil {
		return Null, receiver, false, true, err
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return Null, receiver, false, true, fmt.Errorf("Matcher invalid regex: %w", err)
	}
	switch method {
	case "matches":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.matches expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		anchored, err := regexp.Compile("^(?:" + source + ")$")
		if err != nil {
			return Null, receiver, false, true, fmt.Errorf("Matcher.matches invalid regex: %w", err)
		}
		indices := anchored.FindStringSubmatchIndex(input[region.startByte:region.endByte])
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
		offsetRegexIndices(indices, region.startByte)
		matcherSaveMatch(receiver, indices)
		receiver.Fields["index"] = Int(int64(region.endByte))
		return Bool(true), receiver, true, true, nil
	case "lookingAt":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.lookingAt expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		anchored, err := regexp.Compile("^(?:" + source + ")")
		if err != nil {
			return Null, receiver, false, true, fmt.Errorf("Matcher.lookingAt invalid regex: %w", err)
		}
		indices := anchored.FindStringSubmatchIndex(input[region.startByte:region.endByte])
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
		offsetRegexIndices(indices, region.startByte)
		matcherSaveMatch(receiver, indices)
		receiver.Fields["index"] = Int(int64(indices[1]))
		return Bool(true), receiver, true, true, nil
	case "find":
		if len(args) != 0 && (len(args) != 1 || args[0].Kind != ValueInt) {
			return Null, receiver, false, true, fmt.Errorf("Matcher.find expects optional Integer start")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		startByte := region.startByte
		if len(args) == 1 {
			startRune := int(args[0].Int)
			if startRune < region.startRune || startRune > region.endRune {
				return Null, receiver, false, true, fmt.Errorf("Matcher.find start out of region")
			}
			startByte, err = byteIndexForRuneIndex(input, startRune)
			if err != nil {
				return Null, receiver, false, true, fmt.Errorf("Matcher.find %w", err)
			}
		} else if index, ok := receiver.Fields["index"]; ok && index.Kind == ValueInt {
			startByte = int(index.Int)
		}
		if startByte < region.startByte {
			return Null, receiver, false, true, fmt.Errorf("Matcher.find start before region")
		}
		if startByte > region.endByte {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			return Bool(false), receiver, true, true, nil
		}
		indices := re.FindStringSubmatchIndex(input[startByte:region.endByte])
		if indices == nil {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			return Bool(false), receiver, true, true, nil
		}
		offsetRegexIndices(indices, startByte)
		matcherSaveMatch(receiver, indices)
		next := indices[1]
		if indices[0] == indices[1] {
			next = nextRegexSearchIndex(input, next)
		}
		if next > region.endByte {
			next = region.endByte + 1
		}
		receiver.Fields["index"] = Int(int64(next))
		return Bool(true), receiver, true, true, nil
	case "group":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.group", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		group, err := matcherGroupValue(receiver, input, groupIndex)
		return group, receiver, false, true, err
	case "groupCount":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.groupCount expects 0 arguments")
		}
		return Int(int64(re.NumSubexp())), receiver, false, true, nil
	case "start":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.start", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		start, _, err := matcherGroupBounds(receiver, input, groupIndex)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(start)), receiver, false, true, nil
	case "end":
		groupIndex, err := matcherOptionalGroupIndex("Matcher.end", args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		_, end, err := matcherGroupBounds(receiver, input, groupIndex)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(end)), receiver, false, true, nil
	case "replaceAll":
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		replaced, err := matcherReplace("Matcher.replaceAll", re, input, region, args, true)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(region.startByte))
		return String(replaced), receiver, true, true, nil
	case "replaceFirst":
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		replaced, err := matcherReplace("Matcher.replaceFirst", re, input, region, args, false)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(region.startByte))
		return String(replaced), receiver, true, true, nil
	case "reset":
		if len(args) != 0 && (len(args) != 1 || args[0].Kind != ValueString) {
			return Null, receiver, false, true, fmt.Errorf("Matcher.reset expects optional input String")
		}
		if len(args) == 1 {
			receiver.Fields["input"] = args[0]
		}
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(0)
		input := receiver.Fields["input"]
		receiver.Fields["regionStart"] = Int(0)
		receiver.Fields["regionEnd"] = Int(int64(utf8.RuneCountInString(input.Text)))
		return receiver, receiver, true, true, nil
	case "region":
		if len(args) != 2 || args[0].Kind != ValueInt || args[1].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Matcher.region expects start and end Integers")
		}
		start, end := int(args[0].Int), int(args[1].Int)
		if err := validateMatcherRegion(input, start, end); err != nil {
			return Null, receiver, false, true, err
		}
		startByte, _ := byteIndexForRuneIndex(input, start)
		receiver.Fields["regionStart"] = args[0]
		receiver.Fields["regionEnd"] = args[1]
		matcherClearMatch(receiver)
		receiver.Fields["index"] = Int(int64(startByte))
		return receiver, receiver, true, true, nil
	case "regionStart":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.regionStart expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(region.startRune)), receiver, false, true, nil
	case "regionEnd":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.regionEnd expects 0 arguments")
		}
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(region.endRune)), receiver, false, true, nil
	case "usePattern":
		if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Pattern" {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern expects Pattern")
		}
		source, ok := args[0].Fields["source"]
		if !ok || source.Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern Pattern missing source")
		}
		if _, err := regexp.Compile(source.Text); err != nil {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern invalid regex: %w", err)
		}
		receiver.Fields["source"] = source
		matcherClearMatch(receiver)
		region, err := matcherRegion(receiver, input)
		if err != nil {
			return Null, receiver, false, true, err
		}
		receiver.Fields["index"] = Int(int64(region.startByte))
		return receiver, receiver, true, true, nil
	case "appendReplacement", "appendTail":
		return Null, receiver, false, true, unsupportedCallError("Matcher." + method + " requires Java StringBuffer append semantics")
	case "hasAnchoringBounds":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.hasAnchoringBounds expects 0 arguments")
		}
		return Bool(matcherBoolField(receiver, "anchoringBounds", true)), receiver, false, true, nil
	case "hasTransparentBounds":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.hasTransparentBounds expects 0 arguments")
		}
		return Bool(matcherBoolField(receiver, "transparentBounds", false)), receiver, false, true, nil
	case "useAnchoringBounds":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Matcher.useAnchoringBounds expects Boolean")
		}
		receiver.Fields["anchoringBounds"] = args[0]
		return receiver, receiver, true, true, nil
	case "useTransparentBounds":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Matcher.useTransparentBounds expects Boolean")
		}
		receiver.Fields["transparentBounds"] = args[0]
		return receiver, receiver, true, true, nil
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

func matcherClearMatch(matcher Value) {
	matcher.Fields["groups"] = Null
}

func matcherSaveMatch(matcher Value, indices []int) {
	groups := make([]Value, 0, len(indices))
	for _, index := range indices {
		groups = append(groups, Int(int64(index)))
	}
	matcher.Fields["groups"] = List(groups...)
}

func matcherBoolField(matcher Value, name string, defaultValue bool) bool {
	value, ok := matcher.Fields[name]
	if !ok || value.Kind != ValueBool {
		return defaultValue
	}
	return value.Bool
}

type matcherRegionBounds struct {
	startRune int
	endRune   int
	startByte int
	endByte   int
}

func matcherRegion(matcher Value, input string) (matcherRegionBounds, error) {
	inputRunes := utf8.RuneCountInString(input)
	start := 0
	end := inputRunes
	if value, ok := matcher.Fields["regionStart"]; ok {
		if value.Kind != ValueInt {
			return matcherRegionBounds{}, fmt.Errorf("Matcher stored invalid region start")
		}
		start = int(value.Int)
	}
	if value, ok := matcher.Fields["regionEnd"]; ok {
		if value.Kind != ValueInt {
			return matcherRegionBounds{}, fmt.Errorf("Matcher stored invalid region end")
		}
		end = int(value.Int)
	}
	if err := validateMatcherRegion(input, start, end); err != nil {
		return matcherRegionBounds{}, err
	}
	startByte, err := byteIndexForRuneIndex(input, start)
	if err != nil {
		return matcherRegionBounds{}, err
	}
	endByte, err := byteIndexForRuneIndex(input, end)
	if err != nil {
		return matcherRegionBounds{}, err
	}
	return matcherRegionBounds{startRune: start, endRune: end, startByte: startByte, endByte: endByte}, nil
}

func validateMatcherRegion(input string, start, end int) error {
	inputRunes := utf8.RuneCountInString(input)
	if start < 0 || end < 0 {
		return fmt.Errorf("Matcher.region bounds must be non-negative")
	}
	if start > end {
		return fmt.Errorf("Matcher.region start must be less than or equal to end")
	}
	if end > inputRunes {
		return fmt.Errorf("Matcher.region end out of range")
	}
	return nil
}

func byteIndexForRuneIndex(input string, runeIndex int) (int, error) {
	if runeIndex < 0 {
		return 0, fmt.Errorf("rune index must be non-negative")
	}
	if runeIndex == 0 {
		return 0, nil
	}
	count := 0
	for byteIndex := range input {
		if count == runeIndex {
			return byteIndex, nil
		}
		count++
	}
	if count == runeIndex {
		return len(input), nil
	}
	return 0, fmt.Errorf("rune index out of range")
}

func offsetRegexIndices(indices []int, offset int) {
	for i := range indices {
		if indices[i] >= 0 {
			indices[i] += offset
		}
	}
}

func matcherOptionalGroupIndex(name string, args []Value) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args) != 1 || args[0].Kind != ValueInt {
		return 0, fmt.Errorf("%s expects optional Integer groupIndex", name)
	}
	if args[0].Int < 0 {
		return 0, fmt.Errorf("%s groupIndex must be non-negative", name)
	}
	return int(args[0].Int), nil
}

func matcherGroupValue(matcher Value, input string, groupIndex int) (Value, error) {
	start, end, err := matcherGroupByteBounds(matcher, groupIndex)
	if err != nil {
		return Null, err
	}
	if start < 0 || end < 0 {
		return Null, nil
	}
	return String(input[start:end]), nil
}

func matcherGroupBounds(matcher Value, input string, groupIndex int) (int, int, error) {
	start, end, err := matcherGroupByteBounds(matcher, groupIndex)
	if err != nil {
		return 0, 0, err
	}
	if start < 0 || end < 0 {
		return -1, -1, nil
	}
	return utf8.RuneCountInString(input[:start]), utf8.RuneCountInString(input[:end]), nil
}

func matcherGroupByteBounds(matcher Value, groupIndex int) (int, int, error) {
	groups, ok := matcher.Fields["groups"]
	if !ok || groups.Kind != ValueList {
		return 0, 0, fmt.Errorf("Matcher group access called before a successful match")
	}
	offset := groupIndex * 2
	if offset+1 >= len(groups.List) {
		return 0, 0, fmt.Errorf("Matcher groupIndex out of range")
	}
	startValue := groups.List[offset]
	endValue := groups.List[offset+1]
	if startValue.Kind != ValueInt || endValue.Kind != ValueInt {
		return 0, 0, fmt.Errorf("Matcher stored invalid group state")
	}
	return int(startValue.Int), int(endValue.Int), nil
}

func matcherReplace(name string, re *regexp.Regexp, input string, region matcherRegionBounds, args []Value, all bool) (string, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return "", fmt.Errorf("%s expects replacement String", name)
	}
	replacement := javaReplacementToGoTemplate(args[0].Text)
	regionText := input[region.startByte:region.endByte]
	if all {
		return input[:region.startByte] + re.ReplaceAllString(regionText, replacement) + input[region.endByte:], nil
	}
	indices := re.FindStringSubmatchIndex(regionText)
	if indices == nil {
		return input, nil
	}
	var expanded []byte
	expanded = re.ExpandString(expanded, replacement, regionText, indices)
	return input[:region.startByte+indices[0]] + string(expanded) + input[region.startByte+indices[1]:], nil
}

func javaReplacementToGoTemplate(replacement string) string {
	var out strings.Builder
	for i := 0; i < len(replacement); i++ {
		ch := replacement[i]
		if ch == '\\' && i+1 < len(replacement) {
			next := replacement[i+1]
			if next == '$' {
				out.WriteString("$$")
				i++
				continue
			}
			if next == '\\' {
				out.WriteByte(next)
				i++
				continue
			}
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func nextRegexSearchIndex(input string, index int) int {
	if index >= len(input) {
		return len(input) + 1
	}
	_, size := utf8.DecodeRuneInString(input[index:])
	if size <= 0 {
		return index + 1
	}
	return index + size
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
	case "iterator":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("List.iterator expects 0 arguments")
		}
		return collectionIterator(receiver), receiver, false, true, nil
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

func stringStringIntArgs(name string, args []Value) (string, int, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueInt {
		return "", 0, fmt.Errorf("%s expects String and Integer arguments", name)
	}
	return args[0].Text, int(args[1].Int), nil
}

func stringStringTwoIntArgs(name string, args []Value) (string, int, int, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueInt || args[2].Kind != ValueInt {
		return "", 0, 0, fmt.Errorf("%s expects String and two Integer arguments", name)
	}
	return args[0].Text, int(args[1].Int), int(args[2].Int), nil
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

func stringIndexOfAny(text, chars string) int {
	if text == "" || chars == "" {
		return -1
	}
	for i, r := range []rune(text) {
		if strings.ContainsRune(chars, r) {
			return i
		}
	}
	return -1
}

func stringIndexOfAnyBut(text, chars string) int {
	if text == "" {
		return -1
	}
	if chars == "" {
		return 0
	}
	for i, r := range []rune(text) {
		if !strings.ContainsRune(chars, r) {
			return i
		}
	}
	return -1
}

func stringLastIndexOfAny(text, chars string) int {
	if text == "" || chars == "" {
		return -1
	}
	runes := []rune(text)
	for i := len(runes) - 1; i >= 0; i-- {
		if strings.ContainsRune(chars, runes[i]) {
			return i
		}
	}
	return -1
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

func stringOrdinalIndexOf(text, needle string, ordinal int, last bool) int {
	if ordinal <= 0 || needle == "" {
		return -1
	}
	textRunes := []rune(text)
	needleRunes := []rune(needle)
	if len(needleRunes) > len(textRunes) {
		return -1
	}
	seen := 0
	if last {
		for i := len(textRunes) - len(needleRunes); i >= 0; i-- {
			if runesEqual(textRunes[i:i+len(needleRunes)], needleRunes) {
				seen++
				if seen == ordinal {
					return i
				}
			}
		}
		return -1
	}
	for i := 0; i <= len(textRunes)-len(needleRunes); i++ {
		if runesEqual(textRunes[i:i+len(needleRunes)], needleRunes) {
			seen++
			if seen == ordinal {
				return i
			}
		}
	}
	return -1
}

func runesEqual(left, right []rune) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func stringReplaceLiteral(text, target, replacement string, ignoreCase, once bool) string {
	if target == "" {
		return text
	}
	textRunes := []rune(text)
	targetRunes := []rune(target)
	if len(targetRunes) > len(textRunes) {
		return text
	}
	var b strings.Builder
	matched := false
	for i := 0; i < len(textRunes); {
		if (!once || !matched) && i <= len(textRunes)-len(targetRunes) && runeWindowMatches(textRunes[i:i+len(targetRunes)], targetRunes, ignoreCase) {
			b.WriteString(replacement)
			i += len(targetRunes)
			matched = true
			continue
		}
		b.WriteRune(textRunes[i])
		i++
	}
	return b.String()
}

func runeWindowMatches(window, target []rune, ignoreCase bool) bool {
	if len(window) != len(target) {
		return false
	}
	for i := range window {
		if ignoreCase {
			if !strings.EqualFold(string(window[i]), string(target[i])) {
				return false
			}
			continue
		}
		if window[i] != target[i] {
			return false
		}
	}
	return true
}

func stringOverlay(text, overlay string, start, end int) string {
	runes := []rune(text)
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	if end > len(runes) {
		end = len(runes)
	}
	if start > end {
		start, end = end, start
	}
	return string(runes[:start]) + overlay + string(runes[end:])
}

func stringRotate(text string, shift int) string {
	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}
	offset := shift % len(runes)
	if offset < 0 {
		offset += len(runes)
	}
	if offset == 0 {
		return text
	}
	split := len(runes) - offset
	return string(runes[split:]) + string(runes[:split])
}

func stringSwapCase(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsUpper(r):
			b.WriteString(strings.ToLower(string(r)))
		case unicode.IsLower(r):
			b.WriteString(strings.ToUpper(string(r)))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

type stringStripMode int

const (
	stripBoth stringStripMode = iota
	stripStart
	stripEnd
)

func stringStrip(text string, args []Value, mode stringStripMode) (string, error) {
	if len(args) > 1 || (len(args) == 1 && args[0].Kind != ValueString) {
		return "", fmt.Errorf("String strip expects optional stripChars String")
	}
	if len(args) == 0 {
		return stripByPredicate(text, mode, unicode.IsSpace), nil
	}
	chars := args[0].Text
	if chars == "" {
		return text, nil
	}
	return stripByPredicate(text, mode, func(r rune) bool { return strings.ContainsRune(chars, r) }), nil
}

func stripByPredicate(text string, mode stringStripMode, pred func(rune) bool) string {
	runes := []rune(text)
	start := 0
	end := len(runes)
	if mode == stripBoth || mode == stripStart {
		for start < end && pred(runes[start]) {
			start++
		}
	}
	if mode == stripBoth || mode == stripEnd {
		for end > start && pred(runes[end-1]) {
			end--
		}
	}
	return string(runes[start:end])
}

func stringStaticStripAll(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("String.stripAll expects List<String> and optional stripChars String")
	}
	if args[0].Kind != ValueList || (len(args) == 2 && args[1].Kind != ValueString) {
		return Null, fmt.Errorf("String.stripAll expects List<String> and optional stripChars String")
	}
	stripArgs := []Value{}
	if len(args) == 2 {
		stripArgs = []Value{args[1]}
	}
	out := make([]Value, 0, len(args[0].List))
	for _, item := range args[0].List {
		if item.Kind == ValueNull {
			out = append(out, Null)
			continue
		}
		if item.Kind != ValueString {
			return Null, fmt.Errorf("String.stripAll expects List<String>")
		}
		stripped, err := stringStrip(item.Text, stripArgs, stripBoth)
		if err != nil {
			return Null, err
		}
		out = append(out, String(stripped))
	}
	return List(out...), nil
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

func patternSplit(source string, args []Value) ([]string, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
	}
	if args[0].Kind != ValueString || (len(args) == 2 && args[1].Kind != ValueInt) {
		return nil, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
	}
	splitArgs := []Value{String(source)}
	if len(args) == 2 {
		splitArgs = append(splitArgs, args[1])
	}
	parts, err := stringRegexSplit(args[0].Text, splitArgs)
	if err != nil {
		return nil, fmt.Errorf("Pattern.split invalid regex: %w", err)
	}
	return parts, nil
}

func unsupportedJavaRegexFeature(source string) string {
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			if i+1 < len(source) {
				next := source[i+1]
				if next >= '1' && next <= '9' {
					return "Java regex backreferences"
				}
				if next == 'k' {
					return "Java regex named backreferences"
				}
				i++
			}
		case '(':
			if i+2 >= len(source) || source[i+1] != '?' {
				continue
			}
			switch source[i+2] {
			case '<':
				if i+3 < len(source) && (source[i+3] == '=' || source[i+3] == '!') {
					return "Java regex lookbehind"
				}
				return "Java regex named groups"
			case '=', '!':
				return "Java regex lookahead"
			}
		}
	}
	return ""
}

func stringList(parts []string) Value {
	values := make([]Value, 0, len(parts))
	for _, part := range parts {
		values = append(values, String(part))
	}
	return List(values...)
}

func escapeCSV(text string) string {
	if !strings.ContainsAny(text, ",\r\n\"") {
		return text
	}
	return `"` + strings.ReplaceAll(text, `"`, `""`) + `"`
}

func unescapeCSV(text string) string {
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		return strings.ReplaceAll(text[1:len(text)-1], `""`, `"`)
	}
	return text
}

func escapeHTMLCore(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(text)
}

func escapeXML(text string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(text)
}

func unescapeHTMLEntities(text string) string {
	return unescapeCoreEntities(text, false)
}

var htmlNamedEntityReplacements = map[string]string{
	"nbsp":  "\u00a0",
	"copy":  "\u00a9",
	"reg":   "\u00ae",
	"trade": "\u2122",
	"euro":  "\u20ac",
	"ndash": "\u2013",
	"mdash": "\u2014",
}

func unescapeXMLEntities(text string) string {
	return unescapeCoreEntities(text, true)
}

func unescapeCoreEntities(text string, xml bool) string {
	if !strings.Contains(text, "&") {
		return text
	}
	var b strings.Builder
	for i := 0; i < len(text); {
		if text[i] != '&' {
			r, size := utf8.DecodeRuneInString(text[i:])
			b.WriteRune(r)
			i += size
			continue
		}
		semi := strings.IndexByte(text[i:], ';')
		if semi < 0 {
			b.WriteByte(text[i])
			i++
			continue
		}
		semi += i
		entity := text[i+1 : semi]
		if replacement, ok := coreEntityReplacement(entity, xml); ok {
			b.WriteString(replacement)
			i = semi + 1
			continue
		}
		b.WriteString(text[i : semi+1])
		i = semi + 1
	}
	return b.String()
}

func coreEntityReplacement(entity string, xml bool) (string, bool) {
	switch entity {
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	case "amp":
		return "&", true
	case "quot":
		return `"`, true
	case "apos":
		if xml {
			return "'", true
		}
		return "", false
	case "#39":
		return "'", true
	}
	if !xml {
		if replacement, ok := htmlNamedEntityReplacements[entity]; ok {
			return replacement, true
		}
	}
	if strings.HasPrefix(entity, "#") {
		if r, ok := parseNumericEntity(entity[1:]); ok {
			return string(r), true
		}
	}
	return "", false
}

func parseNumericEntity(entity string) (rune, bool) {
	if entity == "" {
		return 0, false
	}
	base := 10
	digits := entity
	if strings.HasPrefix(entity, "x") || strings.HasPrefix(entity, "X") {
		base = 16
		digits = entity[1:]
	}
	if digits == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(digits, base, 32)
	if err != nil || value < 0 || value > utf8.MaxRune {
		return 0, false
	}
	return rune(value), true
}

func escapeJavaLike(text string, escapeSingleQuote, escapeSlash bool) string {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '\b':
			b.WriteString(`\b`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		case '"':
			b.WriteString(`\"`)
		case '\'':
			if escapeSingleQuote {
				b.WriteString(`\'`)
			} else {
				b.WriteRune(r)
			}
		case '\\':
			b.WriteString(`\\`)
		case '/':
			if escapeSlash {
				b.WriteString(`\/`)
			} else {
				b.WriteRune(r)
			}
		default:
			if r < 32 || r > 0x7e {
				writeUnicodeEscapes(&b, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func escapeUnicode(text string) string {
	var b strings.Builder
	for _, r := range text {
		if r < 32 || r > 0x7e {
			writeUnicodeEscapes(&b, r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func writeUnicodeEscapes(b *strings.Builder, r rune) {
	for _, unit := range utf16.Encode([]rune{r}) {
		b.WriteString(`\u`)
		b.WriteString(fmt.Sprintf("%04X", unit))
	}
}

func unescapeJavaLike(name, text string) (string, error) {
	var out []rune
	for i := 0; i < len(text); i++ {
		if text[i] != '\\' {
			r, size := utf8.DecodeRuneInString(text[i:])
			out = append(out, r)
			i += size - 1
			continue
		}
		i++
		if i >= len(text) {
			return "", fmt.Errorf("%s trailing escape", name)
		}
		switch text[i] {
		case 'b':
			out = append(out, '\b')
		case 'n':
			out = append(out, '\n')
		case 't':
			out = append(out, '\t')
		case 'f':
			out = append(out, '\f')
		case 'r':
			out = append(out, '\r')
		case '"', '\'', '\\', '/':
			out = append(out, rune(text[i]))
		case 'u':
			units := []uint16{}
			for {
				if i+4 >= len(text) {
					return "", fmt.Errorf("%s invalid unicode escape", name)
				}
				value, err := strconv.ParseUint(text[i+1:i+5], 16, 16)
				if err != nil {
					return "", fmt.Errorf("%s invalid unicode escape", name)
				}
				units = append(units, uint16(value))
				i += 4
				if i+2 >= len(text) || text[i+1] != '\\' || text[i+2] != 'u' {
					break
				}
				i += 2
			}
			out = append(out, utf16.Decode(units)...)
		default:
			out = append(out, rune(text[i]))
		}
	}
	return string(out), nil
}

func formatString(pattern string, args []Value) string {
	return regexp.MustCompile(`\{([0-9]+)\}`).ReplaceAllStringFunc(pattern, func(match string) string {
		inner := match[1 : len(match)-1]
		index, err := strconv.Atoi(inner)
		if err != nil || index < 0 || index >= len(args) {
			return match
		}
		return args[index].String()
	})
}

func stringAbbreviate(text string, args []Value) (string, error) {
	if len(args) == 1 && args[0].Kind == ValueInt {
		return abbreviateRunes([]rune(text), 0, int(args[0].Int))
	}
	if len(args) == 2 && args[0].Kind == ValueInt && args[1].Kind == ValueInt {
		return abbreviateRunes([]rune(text), int(args[0].Int), int(args[1].Int))
	}
	return "", fmt.Errorf("String.abbreviate expects maxWidth or offset and maxWidth")
}

func abbreviateRunes(runes []rune, offset, maxWidth int) (string, error) {
	if maxWidth < 4 {
		return "", fmt.Errorf("String.abbreviate maxWidth must be at least 4")
	}
	if len(runes) <= maxWidth {
		return string(runes), nil
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(runes) {
		offset = len(runes)
	}
	if len(runes)-offset < maxWidth-3 {
		offset = len(runes) - (maxWidth - 3)
	}
	if offset <= 4 {
		return string(runes[:maxWidth-3]) + "...", nil
	}
	if maxWidth < 7 {
		return "", fmt.Errorf("String.abbreviate maxWidth with offset must be at least 7")
	}
	if offset+maxWidth-3 < len(runes) {
		abbreviated, err := abbreviateRunes(runes[offset:], 0, maxWidth-3)
		if err != nil {
			return "", err
		}
		return "..." + abbreviated, nil
	}
	return "..." + string(runes[len(runes)-(maxWidth-3):]), nil
}

func stringDifference(left, right string) string {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	limit := len(leftRunes)
	if len(rightRunes) < limit {
		limit = len(rightRunes)
	}
	for i := 0; i < limit; i++ {
		if leftRunes[i] != rightRunes[i] {
			return string(rightRunes[i:])
		}
	}
	return string(rightRunes[limit:])
}

func commonPrefix(texts []string) string {
	if len(texts) == 0 {
		return ""
	}
	prefix := []rune(texts[0])
	for _, text := range texts[1:] {
		runes := []rune(text)
		limit := len(prefix)
		if len(runes) < limit {
			limit = len(runes)
		}
		i := 0
		for i < limit && prefix[i] == runes[i] {
			i++
		}
		prefix = prefix[:i]
		if len(prefix) == 0 {
			break
		}
	}
	return string(prefix)
}

func levenshteinDistance(left, right string) int {
	a := []rune(left)
	b := []rune(right)
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = minInt(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(b)]
}

func minInt(values ...int) int {
	best := values[0]
	for _, value := range values[1:] {
		if value < best {
			best = value
		}
	}
	return best
}

func splitByCharacterType(text string, camelCase bool) []string {
	runes := []rune(text)
	if len(runes) == 0 {
		return nil
	}
	parts := []string{}
	start := 0
	lastType := characterType(runes[0])
	for i := 1; i < len(runes); i++ {
		currentType := characterType(runes[i])
		if currentType == lastType {
			continue
		}
		if camelCase && lastType == stringCharUpper && currentType == stringCharLower {
			if i-start > 1 {
				parts = append(parts, string(runes[start:i-1]))
				start = i - 1
			}
		} else {
			parts = append(parts, string(runes[start:i]))
			start = i
		}
		lastType = currentType
	}
	parts = append(parts, string(runes[start:]))
	return parts
}

type stringCharType int

const (
	stringCharUpper stringCharType = iota
	stringCharLower
	stringCharDigit
	stringCharSpace
	stringCharOther
)

func characterType(r rune) stringCharType {
	switch {
	case unicode.IsUpper(r):
		return stringCharUpper
	case unicode.IsLower(r):
		return stringCharLower
	case unicode.IsDigit(r):
		return stringCharDigit
	case unicode.IsSpace(r):
		return stringCharSpace
	default:
		return stringCharOther
	}
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

func (vm *VM) callIdMember(receiver Value, method string, args []Value) (Value, bool, error) {
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
	case "to18":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.to18 expects 0 arguments")
		}
		if err := validateApexID(receiver.Text); err != nil {
			return Null, true, err
		}
		if len(receiver.Text) == 18 {
			return String(receiver.Text), true, nil
		}
		return String(apexIDTo18(receiver.Text)), true, nil
	case "getSObjectType":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.getSObjectType expects 0 arguments")
		}
		if err := validateApexID(receiver.Text); err != nil {
			return Null, true, err
		}
		objectName, ok := vm.sObjectNameForIDPrefix(receiver.Text[:3])
		if !ok {
			return Null, true, fmt.Errorf("System.StringException: Invalid id prefix: %s", receiver.Text[:3])
		}
		token := Object("Schema.SObjectType")
		token.Fields["object"] = String(objectName)
		return token, true, nil
	default:
		return Null, false, nil
	}
}

func validateApexID(text string) error {
	if err := validateApexIDShape(text); err != nil {
		return err
	}
	if len(text) == 18 && apexIDTo18(text[:15]) != text {
		return fmt.Errorf("System.StringException: Invalid id: %s", text)
	}
	return nil
}

func validateApexIDShape(text string) error {
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

func apexIDTo18(text string) string {
	checksumChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	out := text[:15]
	for chunk := 0; chunk < 3; chunk++ {
		mask := 0
		for bit := 0; bit < 5; bit++ {
			ch := text[chunk*5+bit]
			if ch >= 'A' && ch <= 'Z' {
				mask |= 1 << bit
			}
		}
		out += string(checksumChars[mask])
	}
	return out
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
	if err := validateApexIDShape(text); err != nil {
		return "", err
	}
	if len(text) != 18 {
		return text, nil
	}
	checksum := strings.ToUpper(text[15:])
	out := []byte(strings.ToLower(text[:15]))
	for chunk := 0; chunk < 3; chunk++ {
		mask, ok := apexIDChecksumMask(checksum[chunk])
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
	return string(out) + checksum, nil
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
