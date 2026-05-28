package vm

import (
	"fmt"
	"html"
	"math"
	"math/big"
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
		method = canonicalStdlibMemberName(method, "format", "intValue", "longValue", "doubleValue", "decimalValue")
		return callIntegerMember(receiver, method, args)
	case ValueString:
		method = canonicalStdlibMemberName(method, stringStdlibMethodNames...)
		value, handled, err := callStringMember(receiver, method, args)
		return value, receiver, false, handled, err
	case ValueDecimal:
		method = canonicalStdlibMemberName(method, "abs", "setScale", "round", "intValue", "longValue", "doubleValue", "format", "toPlainString", "divide", "scale", "precision", "stripTrailingZeros")
		return callDecimalMember(receiver, method, args)
	case ValueList:
		method = canonicalStdlibMemberName(method, "add", "addAll", "clear", "clone", "contains", "deepClone", "get", "getSObjectType", "isEmpty", "iterator", "remove", "set", "size", "sort")
		return callListStdlibMember(receiver, method, args)
	case ValueSet:
		method = canonicalStdlibMemberName(method, "add", "addAll", "clear", "clone", "contains", "containsAll", "deepClone", "isEmpty", "iterator", "remove", "removeAll", "retainAll", "size")
		return callSetStdlibMember(receiver, method, args)
	case ValueMap:
		method = canonicalStdlibMemberName(method, "clear", "clone", "containsKey", "deepClone", "get", "isEmpty", "keySet", "put", "putAll", "remove", "size", "values")
		return callMapStdlibMember(receiver, method, args)
	case ValueObject:
		if isIteratorValue(receiver) {
			method = canonicalStdlibMemberName(method, "hasNext", "next")
			return callIteratorMember(receiver, method, args)
		}
		return Null, receiver, false, false, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func canonicalStdlibMemberName(method string, known ...string) string {
	for _, candidate := range known {
		if strings.EqualFold(method, candidate) {
			return candidate
		}
	}
	return method
}

var stringStdlibMethodNames = []string{
	"abbreviate", "capitalize", "center", "charAt", "codePointAt", "codePointBefore", "codePointCount",
	"commonPrefix", "compareTo", "contains", "containsAny", "containsIgnoreCase", "containsNone",
	"containsOnly", "containsWhitespace", "countMatches", "deleteWhitespace", "difference", "endsWith",
	"endsWithIgnoreCase", "equals", "equalsIgnoreCase", "escapeCsv", "escapeEcmaScript", "escapeHtml3",
	"escapeHtml4", "escapeJava", "escapeUnicode", "escapeXml", "escapeXml10", "escapeXml11", "format",
	"getChars", "getLevenshteinDistance", "hashCode", "indexOf", "indexOfAny", "indexOfAnyBut",
	"indexOfChar", "indexOfDifference", "indexOfIgnoreCase",
	"isAllLowerCase", "isAllUpperCase", "isAlpha", "isAlphaSpace", "isAlphanumeric", "isAlphanumericSpace",
	"isAsciiPrintable", "isBlank", "isEmpty", "isNotBlank", "isNotEmpty", "isNumeric", "isNumericSpace",
	"isWhitespace", "lastIndexOf", "lastIndexOfAny", "lastIndexOfChar", "lastIndexOfIgnoreCase",
	"lastOrdinalIndexOf", "left", "leftPad", "length", "mid", "normalizeSpace", "offsetByCodePoints",
	"ordinalIndexOf", "overlay", "remove", "removeEnd", "removeEndIgnoreCase",
	"removeIgnoreCase", "removeStart", "removeStartIgnoreCase", "repeat", "replace", "replaceAll",
	"replaceFirst", "replaceIgnoreCase", "replaceOnce", "reverse", "right", "rightPad", "rotate", "split",
	"splitByCharacterType", "splitByCharacterTypeCamelCase", "startsWith", "startsWithIgnoreCase", "strip",
	"stripEnd", "stripHtmlTags", "stripStart", "stripToEmpty", "stripToNull", "substring", "substringAfter",
	"substringAfterLast", "substringBefore", "substringBeforeLast", "substringBetween", "swapCase",
	"toCharArray", "toLowerCase", "toString", "toUpperCase", "trim", "uncapitalize", "unescapeCsv",
	"unescapeEcmaScript", "unescapeHtml3", "unescapeHtml4", "unescapeJava", "unescapeUnicode",
	"unescapeXml", "unescapeXml10", "unescapeXml11", "valueOf",
}

func callIntegerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "format":
		if len(args) != 0 {
			return Null, receiver, false, true, unsupportedCallError("Integer/Long.format locale/pattern overloads")
		}
		return String(formatIntegerWithGrouping(receiver.Int)), receiver, false, true, nil
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
		value := Decimal(rounded)
		value.Text = strconv.FormatFloat(rounded, 'f', int(args[0].Int), 64)
		return value, receiver, false, true, nil
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
			return Null, receiver, false, true, unsupportedCallError("Decimal/Double.format locale/pattern overloads")
		}
		if err := ensureFiniteDecimal("Decimal.format", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		return String(formatDecimalWithGrouping(receiver.Decimal)), receiver, false, true, nil
	case "toPlainString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.toPlainString expects 0 arguments")
		}
		if err := ensureFiniteDecimal("Decimal.toPlainString", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		if receiver.Text != "" {
			return String(receiver.Text), receiver, false, true, nil
		}
		return String(strconv.FormatFloat(receiver.Decimal, 'f', -1, 64)), receiver, false, true, nil
	case "scale":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.scale expects 0 arguments")
		}
		return Int(int64(decimalScale(receiver))), receiver, false, true, nil
	case "precision":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.precision expects 0 arguments")
		}
		return Int(int64(decimalPrecision(receiver))), receiver, false, true, nil
	case "stripTrailingZeros":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.stripTrailingZeros expects 0 arguments")
		}
		if err := ensureFiniteDecimal("Decimal.stripTrailingZeros", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		text := decimalPlainText(receiver)
		if dot := strings.IndexByte(text, '.'); dot >= 0 {
			text = strings.TrimRight(text, "0")
			text = strings.TrimRight(text, ".")
		}
		if text == "" || text == "-" {
			text = "0"
		}
		value := Decimal(receiver.Decimal)
		value.Text = text
		return value, receiver, false, true, nil
	case "divide":
		if len(args) < 2 || len(args) > 3 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.divide expects divisor, scale, and optional RoundingMode")
		}
		divisor, ok := decimalOperand(args[0])
		if !ok {
			return Null, receiver, false, true, fmt.Errorf("Decimal.divide expects Decimal divisor")
		}
		if args[1].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Decimal.divide expects Integer scale")
		}
		if args[1].Int < 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.divide expects non-negative scale")
		}
		mode := "HALF_UP"
		if len(args) == 3 {
			parsedMode, err := decimalRoundingMode(args[2])
			if err != nil {
				return Null, receiver, false, true, err
			}
			mode = parsedMode
		}
		result, err := decimalDivide(receiver.Decimal, divisor, args[1].Int, mode)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Decimal(result), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func decimalRoundingMode(value Value) (string, error) {
	if value.Kind != ValueObject || (value.Type != "RoundingMode" && value.Type != "System.RoundingMode") {
		return "", fmt.Errorf("Decimal rounding expects RoundingMode")
	}
	switch value.Text {
	case "UP", "DOWN", "CEILING", "FLOOR", "HALF_UP", "HALF_DOWN", "HALF_EVEN", "UNNECESSARY":
		return value.Text, nil
	default:
		return "", fmt.Errorf("unsupported Decimal rounding mode %q", value.Text)
	}
}

func decimalOperand(value Value) (float64, bool) {
	switch value.Kind {
	case ValueDecimal:
		return value.Decimal, true
	case ValueInt:
		return float64(value.Int), true
	default:
		return 0, false
	}
}

func decimalPlainText(value Value) string {
	text := strings.TrimSpace(value.Text)
	if text != "" {
		return text
	}
	return strconv.FormatFloat(value.Decimal, 'f', -1, 64)
}

func decimalScale(value Value) int {
	text := decimalPlainText(value)
	if exponent := strings.IndexAny(text, "eE"); exponent >= 0 {
		text = text[:exponent]
	}
	dot := strings.IndexByte(text, '.')
	if dot < 0 {
		return 0
	}
	return len(text) - dot - 1
}

func decimalPrecision(value Value) int {
	text := decimalPlainText(value)
	if exponent := strings.IndexAny(text, "eE"); exponent >= 0 {
		text = text[:exponent]
	}
	digits := 0
	seenNonZero := false
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			continue
		}
		if ch != '0' {
			seenNonZero = true
		}
		if seenNonZero {
			digits++
		}
	}
	if digits == 0 {
		return 1
	}
	return digits
}

func formatIntegerWithGrouping(value int64) string {
	sign := ""
	text := strconv.FormatInt(value, 10)
	if strings.HasPrefix(text, "-") {
		sign = "-"
		text = text[1:]
	}
	return sign + addThousandsSeparators(text)
}

func formatDecimalWithGrouping(value float64) string {
	text := strconv.FormatFloat(value, 'f', -1, 64)
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign = "-"
		text = text[1:]
	}
	whole := text
	fraction := ""
	if dot := strings.IndexByte(text, '.'); dot >= 0 {
		whole = text[:dot]
		fraction = text[dot:]
	}
	return sign + addThousandsSeparators(whole) + fraction
}

func addThousandsSeparators(text string) string {
	if len(text) <= 3 {
		return text
	}
	first := len(text) % 3
	if first == 0 {
		first = 3
	}
	var out strings.Builder
	out.WriteString(text[:first])
	for i := first; i < len(text); i += 3 {
		out.WriteByte(',')
		out.WriteString(text[i : i+3])
	}
	return out.String()
}

func roundDecimalToScale(callee string, value float64, scaleValue int64, mode string) (float64, error) {
	const maxLocalScale int64 = 15
	if err := ensureFiniteDecimal(callee, value); err != nil {
		return 0, err
	}
	if scaleValue > maxLocalScale || scaleValue < -maxLocalScale {
		return 0, unsupportedCallError(fmt.Sprintf("%s absolute scale greater than %d is not supported by the local decimal model", callee, maxLocalScale))
	}
	rounded, err := roundLocalDecimalStringToScale(callee, value, scaleValue, mode)
	if err != nil {
		return 0, err
	}
	return rounded, nil
}

func ensureFiniteDecimal(callee string, value float64) error {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return fmt.Errorf("%s value must be finite", callee)
	}
	return nil
}

func roundLocalDecimalStringToScale(callee string, value float64, scaleValue int64, mode string) (float64, error) {
	rat := new(big.Rat)
	if _, ok := rat.SetString(strconv.FormatFloat(value, 'f', -1, 64)); !ok {
		return 0, fmt.Errorf("%s value cannot be represented by local decimal model", callee)
	}
	absScale := scaleValue
	if absScale < 0 {
		absScale = -absScale
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(absScale), nil)
	factorRat := new(big.Rat).SetInt(factor)
	scaled := new(big.Rat)
	if scaleValue >= 0 {
		scaled.Mul(rat, factorRat)
	} else {
		scaled.Quo(rat, factorRat)
	}
	rounded, err := roundScaledRat(callee, scaled, mode)
	if err != nil {
		return 0, err
	}
	resultRat := new(big.Rat)
	if scaleValue >= 0 {
		resultRat.SetFrac(rounded, factor)
	} else {
		resultRat.Mul(new(big.Rat).SetInt(rounded), factorRat)
	}
	result, _ := resultRat.Float64()
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return 0, fmt.Errorf("%s rounded value must be finite", callee)
	}
	return result, nil
}

func roundScaledRat(callee string, value *big.Rat, mode string) (*big.Int, error) {
	num := value.Num()
	den := value.Denom()
	q := new(big.Int).Quo(num, den)
	remainder := new(big.Int).Sub(num, new(big.Int).Mul(new(big.Int).Set(q), den))
	if remainder.Sign() == 0 {
		return q, nil
	}
	sign := num.Sign()
	absRem := new(big.Int).Abs(remainder)
	twiceRem := new(big.Int).Lsh(new(big.Int).Set(absRem), 1)
	cmpHalf := twiceRem.Cmp(den)
	step := big.NewInt(int64(sign))
	switch mode {
	case "UP":
		return q.Add(q, step), nil
	case "DOWN":
		return q, nil
	case "CEILING":
		if sign > 0 {
			return q.Add(q, big.NewInt(1)), nil
		}
		return q, nil
	case "FLOOR":
		if sign < 0 {
			return q.Sub(q, big.NewInt(1)), nil
		}
		return q, nil
	case "HALF_UP":
		if cmpHalf >= 0 {
			return q.Add(q, step), nil
		}
		return q, nil
	case "HALF_DOWN":
		if cmpHalf > 0 {
			return q.Add(q, step), nil
		}
		return q, nil
	case "HALF_EVEN":
		if cmpHalf > 0 || (cmpHalf == 0 && q.Bit(0) == 1) {
			return q.Add(q, step), nil
		}
		return q, nil
	case "UNNECESSARY":
		return nil, fmt.Errorf("%s rounding necessary for RoundingMode.UNNECESSARY", callee)
	default:
		return nil, fmt.Errorf("unsupported Decimal rounding mode %q", mode)
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
	method = canonicalStringMemberMethod(method)
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
	case "escapeXml":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML(receiver.Text)), true, nil
	case "escapeXml10":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML10(receiver.Text)), true, nil
	case "escapeXml11":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(escapeXML11(receiver.Text)), true, nil
	case "unescapeXml":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntityAny)), true, nil
	case "unescapeXml10":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntity10)), true, nil
	case "unescapeXml11":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.%s expects 0 arguments", method)
		}
		return String(unescapeXMLEntities(receiver.Text, xmlEntity11)), true, nil
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
		if len(args) > 1 {
			return Null, true, fmt.Errorf("String.toLowerCase expects 0 or 1 arguments")
		}
		return String(strings.ToLower(receiver.Text)), true, nil
	case "toUpperCase":
		if len(args) > 1 {
			return Null, true, fmt.Errorf("String.toUpperCase expects 0 or 1 arguments")
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
		needle, start, err := stringSearchArgs("String.indexOf", args, 0)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOf(receiver.Text, needle, start))), true, nil
	case "lastIndexOf":
		needle, start, err := stringSearchArgs("String.lastIndexOf", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOf(receiver.Text, needle, start))), true, nil
	case "indexOfChar":
		char, start, err := stringCharSearchArgs("String.indexOfChar", args, 0)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOf(receiver.Text, string(rune(char)), start))), true, nil
	case "lastIndexOfChar":
		char, start, err := stringCharSearchArgs("String.lastIndexOfChar", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOf(receiver.Text, string(rune(char)), start))), true, nil
	case "indexOfIgnoreCase":
		needle, start, err := stringSearchArgs("String.indexOfIgnoreCase", args, 0)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfFold(receiver.Text, needle, start))), true, nil
	case "lastIndexOfIgnoreCase":
		needle, start, err := stringSearchArgs("String.lastIndexOfIgnoreCase", args, utf8.RuneCountInString(receiver.Text))
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringLastIndexOfFold(receiver.Text, needle, start))), true, nil
	case "indexOfDifference":
		other, err := stringArg("String.indexOfDifference", args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(stringIndexOfDifference(receiver.Text, other))), true, nil
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
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replace expects target and replacement Strings")
		}
		if target == "" {
			return receiver, true, nil
		}
		return String(strings.ReplaceAll(receiver.Text, target, replacement)), true, nil
	case "replaceOnce":
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replaceOnce expects target and replacement Strings")
		}
		return String(stringReplaceLiteral(receiver.Text, target, replacement, false, true)), true, nil
	case "replaceIgnoreCase":
		target, replacement, ok := stringReplacementArgs(args)
		if !ok {
			return Null, true, fmt.Errorf("String.replaceIgnoreCase expects target and replacement Strings")
		}
		return String(stringReplaceLiteral(receiver.Text, target, replacement, true, false)), true, nil
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
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Bool(false), true, nil
		}
		other, err := stringArg("String.equalsIgnoreCase", args)
		if err != nil {
			return Null, true, err
		}
		return Bool(strings.EqualFold(receiver.Text, other)), true, nil
	case "equals":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Bool(false), true, nil
		}
		if len(args) != 1 {
			return Null, true, newExceptionError("System.NullPointerException", "String.equals expects 1 argument")
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Id") {
			if text, ok := platformScalarObjectText(args[0]); ok {
				return Bool(apexIDTextEqual(receiver.Text, text)), true, nil
			}
		}
		if strings.EqualFold(receiver.Type, "Id") || strings.EqualFold(args[0].Type, "Id") {
			if other, ok := idValueText(args[0]); ok {
				return Bool(apexIDTextEqual(receiver.Text, other)), true, nil
			}
		}
		return Bool(receiver.Text == args[0].String()), true, nil
	case "hashCode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.hashCode expects 0 arguments")
		}
		return Int(int64(javaStringHashCode(receiver.Text))), true, nil
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
		return Int(int64(runes[index])), true, nil
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
	case "offsetByCodePoints":
		index, offset, err := stringTwoIntArgs("String.offsetByCodePoints", args)
		if err != nil {
			return Null, true, err
		}
		runes := []rune(receiver.Text)
		result := index + offset
		if index < 0 || index > len(runes) || result < 0 || result > len(runes) {
			return Null, true, fmt.Errorf("String.offsetByCodePoints index out of bounds")
		}
		return Int(int64(result)), true, nil
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
			return String(""), true, nil
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
			return String(""), true, nil
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
		distance, err := stringLevenshteinDistance("String.getLevenshteinDistance", receiver.Text, args)
		if err != nil {
			return Null, true, err
		}
		return Int(int64(distance)), true, nil
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
	case "substringBetween":
		return stringSubstringBetween(receiver.Text, args)
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
	case "stripHtmlTags":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("String.stripHtmlTags expects 0 arguments")
		}
		return String(stripHTMLTags(receiver.Text)), true, nil
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
				return String(""), true, nil
			}
			return String(strings.Repeat(receiver.Text, int(args[0].Int))), true, nil
		}
		if len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueInt {
			if args[1].Int < 0 {
				return String(""), true, nil
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

var stringMemberMethodNames = []string{
	"length",
	"contains",
	"containsIgnoreCase",
	"containsAny",
	"containsOnly",
	"containsNone",
	"indexOfAny",
	"indexOfAnyBut",
	"lastIndexOfAny",
	"containsWhitespace",
	"countMatches",
	"escapeCsv",
	"unescapeCsv",
	"escapeHtml3",
	"escapeHtml4",
	"unescapeHtml3",
	"unescapeHtml4",
	"escapeXml",
	"escapeXml10",
	"escapeXml11",
	"unescapeXml",
	"unescapeXml10",
	"unescapeXml11",
	"escapeJava",
	"unescapeJava",
	"escapeEcmaScript",
	"unescapeEcmaScript",
	"escapeUnicode",
	"unescapeUnicode",
	"startsWith",
	"startsWithIgnoreCase",
	"endsWith",
	"endsWithIgnoreCase",
	"toLowerCase",
	"toUpperCase",
	"trim",
	"capitalize",
	"uncapitalize",
	"indexOf",
	"lastIndexOf",
	"ordinalIndexOf",
	"lastOrdinalIndexOf",
	"replace",
	"replaceOnce",
	"replaceIgnoreCase",
	"replaceAll",
	"replaceFirst",
	"remove",
	"removeIgnoreCase",
	"removeStart",
	"removeStartIgnoreCase",
	"removeEnd",
	"removeEndIgnoreCase",
	"split",
	"equalsIgnoreCase",
	"equals",
	"hashCode",
	"compareTo",
	"substring",
	"indexOfChar",
	"lastIndexOfChar",
	"indexOfIgnoreCase",
	"lastIndexOfIgnoreCase",
	"indexOfDifference",
	"offsetByCodePoints",
	"charAt",
	"codePointAt",
	"codePointBefore",
	"codePointCount",
	"getChars",
	"toCharArray",
	"left",
	"right",
	"leftPad",
	"rightPad",
	"center",
	"mid",
	"reverse",
	"overlay",
	"rotate",
	"swapCase",
	"abbreviate",
	"difference",
	"commonPrefix",
	"getLevenshteinDistance",
	"splitByCharacterType",
	"splitByCharacterTypeCamelCase",
	"substringAfter",
	"substringAfterLast",
	"substringBefore",
	"substringBeforeLast",
	"substringBetween",
	"deleteWhitespace",
	"strip",
	"stripStart",
	"stripEnd",
	"stripHtmlTags",
	"stripToNull",
	"stripToEmpty",
	"normalizeSpace",
	"isWhitespace",
	"isAlpha",
	"isAlphaSpace",
	"isAlphanumeric",
	"isAlphanumericSpace",
	"isNumeric",
	"isNumericSpace",
	"isAllLowerCase",
	"isAllUpperCase",
	"isAsciiPrintable",
	"repeat",
}

func canonicalStringMemberMethod(method string) string {
	for _, name := range stringMemberMethodNames {
		if strings.EqualFold(method, name) {
			return name
		}
	}
	return method
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
		if args[0].Kind == ValueNull {
			return Value{Kind: ValueNull, Type: "String"}, nil
		}
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Date") {
			text, err := stringValueOfDate(args[0])
			if err != nil {
				return Null, err
			}
			return String(text), nil
		}
		if idText, ok := typedIDValueText(args[0]); ok {
			return String(displayIDText(idText)), nil
		}
		return String(args[0].String()), nil
	case "String.join":
		if len(args) != 2 || args[1].Kind != ValueString {
			return Null, fmt.Errorf("String.join expects List or Set and separator String")
		}
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
		}
		if args[0].Kind != ValueList && args[0].Kind != ValueSet {
			return Null, fmt.Errorf("String.join expects List or Set and separator String")
		}
		values := args[0].List
		if args[0].Kind == ValueSet {
			values = args[0].Set
		}
		parts := make([]string, 0, len(values))
		for _, item := range values {
			if item.Kind == ValueNull {
				parts = append(parts, "")
				continue
			}
			parts = append(parts, item.String())
		}
		return String(strings.Join(parts, args[1].Text)), nil
	case "String.format":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueList {
			return Null, fmt.Errorf("String.format expects format String and List arguments")
		}
		formatted, err := formatString(args[0].Text, args[1].List, func(value Value) (string, error) {
			return value.String(), nil
		})
		if err != nil {
			return Null, err
		}
		return String(formatted), nil
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
		if len(args) != 2 && len(args) != 3 {
			return Null, fmt.Errorf("String.getLevenshteinDistance expects two Strings and optional threshold")
		}
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("String.getLevenshteinDistance expects left String")
		}
		distance, err := stringLevenshteinDistance("String.getLevenshteinDistance", args[0].Text, args[1:])
		if err != nil {
			return Null, err
		}
		return Int(int64(distance)), nil
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
		if len(args) != 1 {
			return Null, fmt.Errorf("String.escapeSingleQuotes expects String argument")
		}
		if args[0].Kind == ValueNull {
			return Null, nil
		}
		return String(strings.ReplaceAll(args[0].String(), "'", "\\'")), nil
	case "String.toLowerCase", "String.toUpperCase":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, fmt.Errorf("%s expects String argument", callee)
		}
		if callee == "String.toLowerCase" {
			return String(strings.ToLower(args[0].Text)), nil
		}
		return String(strings.ToUpper(args[0].Text)), nil
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
		case ValueNull:
			if integerValueOfNullReturnsNull(args[0]) {
				return Null, nil
			}
			return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
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
				return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s invalid integer %q", callee, args[0].Text))
			}
			return Int(parsed), nil
		default:
			return Null, fmt.Errorf("%s expects String or numeric argument", callee)
		}
	case "Long.valueOf":
		switch args[0].Kind {
		case ValueNull:
			return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
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
				return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s invalid integer %q", callee, args[0].Text))
			}
			return Int(parsed), nil
		default:
			return Null, fmt.Errorf("%s expects String or numeric argument", callee)
		}
	case "Decimal.valueOf", "Double.valueOf":
		if args[0].Kind == ValueNull {
			return Null, newExceptionError("System.NullPointerException", "Argument cannot be null.")
		}
		switch args[0].Kind {
		case ValueDecimal:
			return args[0], nil
		case ValueInt:
			return Decimal(float64(args[0].Int)), nil
		case ValueString:
			text := strings.TrimSpace(args[0].Text)
			parsed, err := strconv.ParseFloat(text, 64)
			if err != nil {
				return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s invalid decimal %q", callee, args[0].Text))
			}
			if math.IsInf(parsed, 0) || math.IsNaN(parsed) {
				return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s invalid finite decimal %q", callee, args[0].Text))
			}
			value := Decimal(parsed)
			value.Text = text
			return value, nil
		default:
			return Null, newExceptionError("System.TypeException", fmt.Sprintf("%s expects String or numeric argument", callee))
		}
	default:
		return Null, unsupportedCallError(callee)
	}
}

func integerValueOfNullReturnsNull(value Value) bool {
	typeName := strings.TrimSpace(value.Type)
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		typeName = rest
	}
	switch typeName {
	case "Integer", "Decimal", "Object":
		return true
	default:
		return false
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

func roundHalfEven(n float64) float64 {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return n
	}
	t := math.Trunc(n)
	frac := math.Abs(n - t)
	if frac < 0.5 {
		return t
	}
	if frac > 0.5 {
		if n >= 0 {
			return t + 1
		}
		return t - 1
	}
	// Exactly 0.5 — round to even
	if math.Mod(math.Abs(t), 2) == 0 {
		return t
	}
	if n >= 0 {
		return t + 1
	}
	return t - 1
}

func decimalDivide(dividend, divisor float64, scale int64, mode string) (float64, error) {
	const maxLocalScale int64 = 15
	if err := ensureFiniteDecimal("Decimal.divide", dividend); err != nil {
		return 0, err
	}
	if err := ensureFiniteDecimal("Decimal.divide", divisor); err != nil {
		return 0, err
	}
	if divisor == 0 {
		return 0, fmt.Errorf("Decimal.divide division by zero")
	}
	if scale > maxLocalScale {
		return 0, unsupportedCallError(fmt.Sprintf("Decimal.divide scale greater than %d is not supported by the local decimal model", maxLocalScale))
	}
	divRat := new(big.Rat)
	if _, ok := divRat.SetString(strconv.FormatFloat(dividend, 'f', -1, 64)); !ok {
		return 0, fmt.Errorf("Decimal.divide dividend cannot be represented")
	}
	divsRat := new(big.Rat)
	if _, ok := divsRat.SetString(strconv.FormatFloat(divisor, 'f', -1, 64)); !ok {
		return 0, fmt.Errorf("Decimal.divide divisor cannot be represented")
	}
	result := new(big.Rat).Quo(divRat, divsRat)
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(scale), nil)
	scaled := new(big.Rat).Mul(result, new(big.Rat).SetInt(factor))
	rounded, err := roundScaledRat("Decimal.divide", scaled, mode)
	if err != nil {
		return 0, err
	}
	resultRat := new(big.Rat).SetFrac(rounded, factor)
	f, _ := resultRat.Float64()
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, fmt.Errorf("Decimal.divide result must be finite")
	}
	return f, nil
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

const (
	patternFlagUnixLines             int64 = 1
	patternFlagCaseInsensitive       int64 = 2
	patternFlagComments              int64 = 4
	patternFlagMultiline             int64 = 8
	patternFlagLiteral               int64 = 16
	patternFlagDotall                int64 = 32
	patternFlagUnicodeCase           int64 = 64
	patternFlagCanonEq               int64 = 128
	patternFlagUnicodeCharacterClass int64 = 256
)

const patternSupportedFlags = patternFlagCaseInsensitive | patternFlagMultiline | patternFlagLiteral | patternFlagDotall | patternFlagUnicodeCase

func patternCompile(args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("Pattern.compile expects regex String and optional Integer flags")
	}
	if args[0].Kind != ValueString || (len(args) == 2 && args[1].Kind != ValueInt) {
		return Null, fmt.Errorf("Pattern.compile expects regex String and optional Integer flags")
	}
	flags := int64(0)
	if len(args) == 2 {
		flags = args[1].Int
	}
	regexpSource, lookaheadSource, err := compilePatternSourceWithMetadata("Pattern.compile", args[0].Text, flags)
	if err != nil {
		return Null, err
	}
	if _, err := regexp.Compile(regexpSource); err != nil {
		return Null, newPatternSyntaxExceptionError(args[0].Text, err)
	}
	if lookaheadSource != "" {
		if _, err := regexp.Compile("^(?:" + lookaheadSource + ")"); err != nil {
			return Null, newPatternSyntaxExceptionError(args[0].Text, err)
		}
	}
	pattern := Object("Pattern")
	pattern.Fields["source"] = args[0]
	pattern.Fields["regexpSource"] = String(regexpSource)
	if lookaheadSource != "" {
		pattern.Fields["lookaheadSource"] = String(lookaheadSource)
	}
	pattern.Fields["flags"] = Int(flags)
	return pattern, nil
}

func patternMatches(args []Value) (Value, error) {
	if len(args) != 2 {
		return Null, fmt.Errorf("Pattern.matches expects regex and input Strings")
	}
	pattern, err := stringArg("Pattern.matches", args[:1])
	if err != nil {
		return Null, err
	}
	input, err := stringArg("Pattern.matches", args[1:])
	if err != nil {
		return Null, err
	}
	source, positiveLookaheads, negativeLookaheads, err := compilePatternMatchesSource(pattern)
	if err != nil {
		return Null, err
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return Null, newPatternSyntaxExceptionError(pattern, err)
	}
	indices := re.FindStringIndex(input)
	matched := indices != nil && indices[0] == 0 && indices[1] == len(input)
	if matched {
		for _, lookahead := range positiveLookaheads {
			if !regexLookaheadMatches(lookahead, input, 0) {
				matched = false
				break
			}
		}
	}
	if matched {
		for _, lookahead := range negativeLookaheads {
			if negativeRegexLookaheadMatches(lookahead, input) {
				matched = false
				break
			}
		}
	}
	return Bool(matched), nil
}

func newPatternSyntaxExceptionError(pattern string, err error) error {
	description := err.Error()
	value := Object("PatternSyntaxException")
	value.Fields["message"] = String(description)
	value.Fields["description"] = String(description)
	value.Fields["pattern"] = String(pattern)
	value.Fields["index"] = Int(-1)
	return &apexThrowError{value: value}
}

func patternQuote(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("Pattern.quote expects String")
	}
	return String(javaPatternQuote(args[0].Text)), nil
}

func javaPatternQuote(text string) string {
	return `\Q` + strings.ReplaceAll(text, `\E`, `\E\\E\Q`) + `\E`
}

func matcherQuoteReplacement(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("Matcher.quoteReplacement expects String")
	}
	quoted := strings.NewReplacer(`\`, `\\`, `$`, `\$`).Replace(args[0].Text)
	return String(quoted), nil
}

func compilePatternSource(callee, source string, flags int64) (string, error) {
	regexpSource, _, err := compilePatternSourceWithMetadata(callee, source, flags)
	return regexpSource, err
}

type regexNegativeLookaheadAssertion struct {
	Prefix    string
	Lookahead string
}

func compilePatternMatchesSource(source string) (string, []string, []regexNegativeLookaheadAssertion, error) {
	regexpSource, err := javaRegexQuoteEscapesToGo(source)
	if err != nil {
		return "", nil, nil, unsupportedCallError("Pattern.matches " + err.Error())
	}
	var positiveLookaheads []string
	regexpSource, positiveLookaheads = stripLeadingPositiveLookaheadAssertions(regexpSource)
	var negativeLookaheads []regexNegativeLookaheadAssertion
	regexpSource, negativeLookaheads = stripNegativeLookaheadAssertions(regexpSource)
	if stripped, _, ok := stripTerminalPositiveLookahead(regexpSource); ok {
		regexpSource = stripped
	}
	regexpSource = stripFixedCountPossessiveQuantifiers(regexpSource)
	if feature := unsupportedJavaRegexFeature(regexpSource); feature != "" {
		return "", nil, nil, unsupportedCallError("Pattern.matches " + feature)
	}
	for _, lookahead := range positiveLookaheads {
		lookahead = stripFixedCountPossessiveQuantifiers(lookahead)
		if feature := unsupportedJavaRegexFeature(lookahead); feature != "" {
			return "", nil, nil, unsupportedCallError("Pattern.matches " + feature)
		}
		if _, err := regexp.Compile("^(?:" + lookahead + ")"); err != nil {
			return "", nil, nil, newPatternSyntaxExceptionError(source, err)
		}
	}
	for _, lookahead := range negativeLookaheads {
		lookahead.Lookahead = stripFixedCountPossessiveQuantifiers(lookahead.Lookahead)
		lookahead.Prefix = stripFixedCountPossessiveQuantifiers(lookahead.Prefix)
		if feature := unsupportedJavaRegexFeature(lookahead.Lookahead); feature != "" {
			return "", nil, nil, unsupportedCallError("Pattern.matches " + feature)
		}
		if _, err := regexp.Compile("^(?:" + lookahead.Prefix + ")"); err != nil {
			return "", nil, nil, newPatternSyntaxExceptionError(source, err)
		}
		if _, err := regexp.Compile("^(?:" + lookahead.Lookahead + ")"); err != nil {
			return "", nil, nil, newPatternSyntaxExceptionError(source, err)
		}
	}
	return regexpSource, positiveLookaheads, negativeLookaheads, nil
}

func compilePatternSourceWithMetadata(callee, source string, flags int64) (string, string, error) {
	if flags < 0 {
		return "", "", unsupportedCallError(callee + " negative regex flags")
	}
	if unsupported := flags &^ patternSupportedFlags; unsupported != 0 {
		return "", "", unsupportedCallError(callee + " " + unsupportedPatternFlagsFeature(unsupported))
	}
	regexpSource := source
	lookaheadSource := ""
	if flags&patternFlagLiteral != 0 {
		regexpSource = regexp.QuoteMeta(source)
	} else {
		converted, err := javaRegexQuoteEscapesToGo(source)
		if err != nil {
			return "", "", unsupportedCallError(callee + " " + err.Error())
		}
		regexpSource = converted
		regexpSource = stripFixedCountPossessiveQuantifiers(regexpSource)
		if stripped, lookahead, ok := stripTerminalPositiveLookahead(regexpSource); ok {
			regexpSource = stripped
			lookaheadSource = stripFixedCountPossessiveQuantifiers(lookahead)
		}
		if feature := unsupportedJavaRegexFeature(regexpSource); feature != "" {
			return "", "", unsupportedCallError(callee + " " + feature)
		}
		if lookaheadSource != "" {
			if feature := unsupportedJavaRegexFeature(lookaheadSource); feature != "" {
				return "", "", unsupportedCallError(callee + " " + feature)
			}
		}
	}
	prefix := patternFlagPrefix(flags)
	if prefix != "" {
		regexpSource = prefix + regexpSource
		if lookaheadSource != "" {
			lookaheadSource = prefix + lookaheadSource
		}
	}
	return regexpSource, lookaheadSource, nil
}

func patternFlagPrefix(flags int64) string {
	var enabled strings.Builder
	if flags&patternFlagCaseInsensitive != 0 {
		enabled.WriteByte('i')
	}
	if flags&patternFlagMultiline != 0 {
		enabled.WriteByte('m')
	}
	if flags&patternFlagDotall != 0 {
		enabled.WriteByte('s')
	}
	if enabled.Len() == 0 {
		return ""
	}
	return "(?" + enabled.String() + ")"
}

func unsupportedPatternFlagsFeature(flags int64) string {
	names := []string{}
	if flags&patternFlagUnixLines != 0 {
		names = append(names, "UNIX_LINES")
	}
	if flags&patternFlagComments != 0 {
		names = append(names, "COMMENTS")
	}
	if flags&patternFlagCanonEq != 0 {
		names = append(names, "CANON_EQ")
	}
	if flags&patternFlagUnicodeCharacterClass != 0 {
		names = append(names, "UNICODE_CHARACTER_CLASS")
	}
	known := patternFlagUnixLines | patternFlagComments | patternFlagCanonEq | patternFlagUnicodeCharacterClass
	if unknown := flags &^ (patternSupportedFlags | known); unknown != 0 {
		names = append(names, fmt.Sprintf("unknown flags 0x%x", unknown))
	}
	if len(names) == 0 {
		return "unsupported regex flags"
	}
	return "unsupported regex flags " + strings.Join(names, ",")
}

func patternRegexpSource(pattern Value) (string, error) {
	if regexpSource, ok := pattern.Fields["regexpSource"]; ok {
		if regexpSource.Kind != ValueString {
			return "", fmt.Errorf("Pattern stored invalid regex source")
		}
		return regexpSource.Text, nil
	}
	source, ok := pattern.Fields["source"]
	if !ok || source.Kind != ValueString {
		return "", fmt.Errorf("Pattern missing source")
	}
	return source.Text, nil
}

func patternLookaheadSource(pattern Value) string {
	lookaheadSource, ok := pattern.Fields["lookaheadSource"]
	if !ok || lookaheadSource.Kind != ValueString {
		return ""
	}
	return lookaheadSource.Text
}

func callPatternMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "matches", "matcher", "pattern", "split")
	switch method {
	case "matches":
		value, err := patternMatches(args)
		return value, receiver, false, true, err
	case "matcher":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Pattern.matcher expects input String")
		}
		regexpSource, err := patternRegexpSource(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		matcher := Object("Matcher")
		matcher.Fields["source"] = String(regexpSource)
		matcher.Fields["patternSource"] = receiver.Fields["source"]
		if lookaheadSource := patternLookaheadSource(receiver); lookaheadSource != "" {
			matcher.Fields["lookaheadSource"] = String(lookaheadSource)
		}
		if flags, ok := receiver.Fields["flags"]; ok {
			matcher.Fields["flags"] = flags
		}
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
		regexpSource, err := patternRegexpSource(receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		parts, err := patternSplit(regexpSource, args)
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
	method = canonicalStdlibMemberName(method,
		"matches", "lookingAt", "find", "group", "groupCount", "start", "end",
		"replaceAll", "replaceFirst", "reset", "region", "regionStart", "regionEnd",
		"usePattern", "hasAnchoringBounds", "hasTransparentBounds", "useAnchoringBounds",
		"useTransparentBounds", "hitEnd", "pattern", "requireEnd",
	)
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
		indices, err := matcherMatchIndices(receiver, source, input, region, matcherOpMatches)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
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
		indices, err := matcherMatchIndices(receiver, source, input, region, matcherOpLookingAt)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			return Bool(false), receiver, true, true, nil
		}
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
			matcherClearMatch(receiver)
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
		indices, err := matcherFindIndices(receiver, re, input, region, startByte)
		if err != nil {
			return Null, receiver, false, true, err
		}
		if indices == nil {
			matcherClearMatch(receiver)
			receiver.Fields["index"] = Int(int64(region.endByte + 1))
			return Bool(false), receiver, true, true, nil
		}
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
		regexpSource, err := patternRegexpSource(args[0])
		if err != nil {
			return Null, receiver, false, true, err
		}
		if _, err := regexp.Compile(regexpSource); err != nil {
			return Null, receiver, false, true, fmt.Errorf("Matcher.usePattern invalid regex: %w", err)
		}
		receiver.Fields["source"] = String(regexpSource)
		receiver.Fields["patternSource"] = source
		if flags, ok := args[0].Fields["flags"]; ok {
			receiver.Fields["flags"] = flags
		} else {
			delete(receiver.Fields, "flags")
		}
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
	case "hitEnd", "requireEnd":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.%s expects 0 arguments", method)
		}
		return Bool(false), receiver, false, true, nil
	case "pattern":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Matcher.pattern expects 0 arguments")
		}
		pattern := Object("Pattern")
		if source, ok := receiver.Fields["patternSource"]; ok && source.Kind == ValueString {
			pattern.Fields["source"] = source
		} else if regexpSource, ok := receiver.Fields["source"]; ok && regexpSource.Kind == ValueString {
			pattern.Fields["source"] = regexpSource
		} else {
			pattern.Fields["source"] = String("")
		}
		pattern.Fields["regexpSource"] = receiver.Fields["source"]
		if lookahead, ok := receiver.Fields["lookaheadSource"]; ok {
			pattern.Fields["lookaheadSource"] = lookahead
		}
		if flags, ok := receiver.Fields["flags"]; ok {
			pattern.Fields["flags"] = flags
		}
		return pattern, receiver, false, true, nil
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

type matcherOp int

const (
	matcherOpMatches matcherOp = iota
	matcherOpLookingAt
)

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

func matcherUsesFullInputBounds(matcher Value) bool {
	return !matcherBoolField(matcher, "anchoringBounds", true) || matcherBoolField(matcher, "transparentBounds", false)
}

func matcherMatchIndices(matcher Value, source, input string, region matcherRegionBounds, op matcherOp) ([]int, error) {
	lookaheadSource := matcherLookaheadSource(matcher)
	if !matcherUsesFullInputBounds(matcher) {
		prefix := "^(?:"
		suffix := ")"
		name := "Matcher.lookingAt"
		if op == matcherOpMatches {
			suffix = ")$"
			name = "Matcher.matches"
		}
		anchored, err := regexp.Compile(prefix + source + suffix)
		if err != nil {
			return nil, fmt.Errorf("%s invalid regex: %w", name, err)
		}
		indices := anchored.FindStringSubmatchIndex(input[region.startByte:region.endByte])
		if indices != nil {
			offsetRegexIndices(indices, region.startByte)
			if !matcherLookaheadMatches(lookaheadSource, input, indices[1]) {
				return nil, nil
			}
		}
		return indices, nil
	}
	re, err := regexp.Compile(source)
	if err != nil {
		return nil, fmt.Errorf("Matcher invalid regex: %w", err)
	}
	for _, indices := range re.FindAllStringSubmatchIndex(input, -1) {
		if len(indices) < 2 || indices[0] < region.startByte {
			continue
		}
		if indices[0] > region.startByte {
			return nil, nil
		}
		if indices[1] > region.endByte {
			return nil, nil
		}
		if op == matcherOpMatches && indices[1] != region.endByte {
			return nil, nil
		}
		if !matcherLookaheadMatches(lookaheadSource, input, indices[1]) {
			return nil, nil
		}
		return indices, nil
	}
	return nil, nil
}

func matcherFindIndices(matcher Value, re *regexp.Regexp, input string, region matcherRegionBounds, startByte int) ([]int, error) {
	lookaheadSource := matcherLookaheadSource(matcher)
	if lookaheadSource != "" {
		return matcherFindIndicesWithTerminalLookahead(matcher, input, region, startByte)
	}
	if !matcherUsesFullInputBounds(matcher) {
		indices := re.FindStringSubmatchIndex(input[startByte:region.endByte])
		if indices != nil {
			offsetRegexIndices(indices, startByte)
		}
		return indices, nil
	}
	if matcherFindCanUseRegionSlice(re.String()) {
		searchStart := startByte
		if searchStart < region.startByte {
			searchStart = region.startByte
		}
		indices := re.FindStringSubmatchIndex(input[searchStart:region.endByte])
		if indices != nil {
			offsetRegexIndices(indices, searchStart)
		}
		return indices, nil
	}
	for _, indices := range re.FindAllStringSubmatchIndex(input, -1) {
		if len(indices) < 2 || indices[0] < startByte || indices[0] < region.startByte {
			continue
		}
		if indices[0] > region.endByte {
			return nil, nil
		}
		if indices[1] <= region.endByte {
			return indices, nil
		}
	}
	return nil, nil
}

func matcherFindCanUseRegionSlice(source string) bool {
	return !strings.ContainsAny(source, "^$") &&
		!strings.Contains(source, `\A`) &&
		!strings.Contains(source, `\z`) &&
		!strings.Contains(source, `\b`) &&
		!strings.Contains(source, `\B`)
}

func matcherFindIndicesWithTerminalLookahead(matcher Value, input string, region matcherRegionBounds, startByte int) ([]int, error) {
	source, inputValue, err := matcherSourceInput(matcher)
	if err != nil {
		return nil, err
	}
	if inputValue != input {
		return nil, fmt.Errorf("Matcher stored inconsistent input")
	}
	lookaheadSource := matcherLookaheadSource(matcher)
	combined, err := regexp.Compile(source + "(?:" + lookaheadSource + ")")
	if err != nil {
		return nil, fmt.Errorf("Matcher invalid regex: %w", err)
	}
	anchored, err := regexp.Compile("^(?:" + source + ")$")
	if err != nil {
		return nil, fmt.Errorf("Matcher invalid regex: %w", err)
	}
	for _, full := range combined.FindAllStringIndex(input, -1) {
		if len(full) < 2 || full[0] < startByte || full[0] < region.startByte {
			continue
		}
		if full[0] > region.endByte {
			return nil, nil
		}
		for end := full[0]; end <= full[1] && end <= region.endByte; end = nextRegexSearchIndex(input, end) {
			if !matcherLookaheadMatches(lookaheadSource, input, end) {
				if end == full[1] {
					break
				}
				continue
			}
			indices := anchored.FindStringSubmatchIndex(input[full[0]:end])
			if indices == nil {
				if end == full[1] {
					break
				}
				continue
			}
			offsetRegexIndices(indices, full[0])
			return indices, nil
		}
	}
	return nil, nil
}

func matcherLookaheadSource(matcher Value) string {
	lookahead, ok := matcher.Fields["lookaheadSource"]
	if !ok || lookahead.Kind != ValueString {
		return ""
	}
	return lookahead.Text
}

func matcherLookaheadMatches(lookaheadSource, input string, endByte int) bool {
	if lookaheadSource == "" {
		return true
	}
	return regexLookaheadMatches(lookaheadSource, input, endByte)
}

func regexLookaheadMatches(lookaheadSource, input string, endByte int) bool {
	if endByte < 0 || endByte > len(input) {
		return false
	}
	re, err := regexp.Compile("^(?:" + lookaheadSource + ")")
	if err != nil {
		return false
	}
	return re.MatchString(input[endByte:])
}

func negativeRegexLookaheadMatches(assertion regexNegativeLookaheadAssertion, input string) bool {
	prefix, err := regexp.Compile("^(?:" + assertion.Prefix + ")")
	if err != nil {
		return false
	}
	prefixMatch := prefix.FindStringIndex(input)
	if prefixMatch == nil {
		return false
	}
	return regexLookaheadMatches(assertion.Lookahead, input, prefixMatch[1])
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
	replacement, err := javaReplacementToGoTemplate(name, args[0].Text, re.NumSubexp())
	if err != nil {
		return "", fmt.Errorf("%s %w", name, err)
	}
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

func javaReplacementToGoTemplate(callee, replacement string, groupCount int) (string, error) {
	var out strings.Builder
	for i := 0; i < len(replacement); i++ {
		ch := replacement[i]
		if ch == '\\' {
			if i+1 >= len(replacement) {
				return "", fmt.Errorf("replacement trailing escape")
			}
			next := replacement[i+1]
			if next == '$' {
				out.WriteString("$$")
				i++
				continue
			}
			out.WriteByte(next)
			i++
			continue
		}
		if ch == '$' {
			if i+1 >= len(replacement) {
				return "", fmt.Errorf("replacement missing group reference after $")
			}
			next := replacement[i+1]
			if next == '{' {
				return "", unsupportedCallError(callee + " replacement named group references")
			}
			if next < '0' || next > '9' {
				return "", fmt.Errorf("replacement invalid group reference")
			}
			group := int(next - '0')
			if group > groupCount {
				return "", fmt.Errorf("replacement groupIndex out of range")
			}
			i += 1
			for i+1 < len(replacement) {
				next = replacement[i+1]
				if next < '0' || next > '9' {
					break
				}
				candidate := group*10 + int(next-'0')
				if candidate > groupCount {
					break
				}
				group = candidate
				i++
			}
			out.WriteString("${")
			out.WriteString(strconv.Itoa(group))
			out.WriteByte('}')
			continue
		}
		out.WriteByte(ch)
	}
	return out.String(), nil
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

func listSObjectTypeName(receiver Value) string {
	if elementType, ok := collectionElementType(receiver.Static); ok && (isCommonSObjectTypeName(elementType) || strings.HasSuffix(elementType, "__c") || strings.HasSuffix(elementType, "__e") || strings.HasSuffix(elementType, "__mdt")) {
		return elementType
	}
	if elementType, ok := collectionElementType(receiver.Runtime); ok && (isCommonSObjectTypeName(elementType) || strings.HasSuffix(elementType, "__c") || strings.HasSuffix(elementType, "__e") || strings.HasSuffix(elementType, "__mdt")) {
		return elementType
	}
	if elementType, ok := collectionElementType(receiver.Type); ok && (isCommonSObjectTypeName(elementType) || strings.HasSuffix(elementType, "__c") || strings.HasSuffix(elementType, "__e") || strings.HasSuffix(elementType, "__mdt") || strings.EqualFold(elementType, "sObject")) {
		if !strings.EqualFold(elementType, "sObject") {
			return elementType
		}
		if len(receiver.List) == 0 {
			return ""
		}
	}
	for _, item := range receiver.List {
		if item.Kind == ValueObject && item.Type != "" && item.Type != "Object" {
			return item.Type
		}
	}
	return ""
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
		if foldedKey, ok := caseInsensitiveStringMapStoredKey(receiver, args[0]); ok {
			key = foldedKey
		}
		value, ok := receiver.Map[key]
		if ok {
			delete(receiver.Map, key)
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

func mapSObjectTypeName(receiver Value) string {
	if valueType := mapConcreteSObjectValueType(receiver.Type); valueType != "" {
		return valueType
	}
	return ""
}

func mapConcreteSObjectValueType(typeName string) string {
	_, valueType, ok := mapTypeArgs(typeName)
	if !ok || strings.EqualFold(valueType, "sObject") {
		return ""
	}
	if isCommonSObjectTypeName(valueType) || strings.HasSuffix(valueType, "__c") || strings.HasSuffix(valueType, "__e") || strings.HasSuffix(valueType, "__mdt") {
		return valueType
	}
	return ""
}

func stringArg(name string, args []Value) (string, error) {
	if len(args) != 1 {
		return "", newExceptionError("System.NullPointerException", fmt.Sprintf("%s expects 1 argument", name))
	}
	if args[0].Kind == ValueNull {
		return "", newExceptionError("System.NullPointerException", fmt.Sprintf("%s expects String argument", name))
	}
	if strings.EqualFold(args[0].Type, "Id") {
		if idText, ok := idValueText(args[0]); ok {
			return idText, nil
		}
	}
	if args[0].Kind != ValueString {
		return "", newExceptionError("System.TypeException", fmt.Sprintf("%s expects String argument", name))
	}
	return args[0].Text, nil
}

func stringReplacementArgs(args []Value) (string, string, bool) {
	if len(args) != 2 {
		return "", "", false
	}
	target, ok := stringReplacementText(args[0])
	if !ok {
		return "", "", false
	}
	replacement, ok := stringReplacementText(args[1])
	if !ok {
		return "", "", false
	}
	return target, replacement, true
}

func stringReplacementText(value Value) (string, bool) {
	if idText, ok := typedIDValueText(value); ok {
		return displayIDText(idText), true
	}
	if value.Kind != ValueString {
		return "", false
	}
	return value.Text, true
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

func stringSearchArgs(name string, args []Value, defaultStart int) (string, int, error) {
	if len(args) != 1 && len(args) != 2 {
		return "", 0, fmt.Errorf("%s expects String and optional Integer arguments", name)
	}
	if args[0].Kind != ValueString || (len(args) == 2 && args[1].Kind != ValueInt) {
		return "", 0, fmt.Errorf("%s expects String and optional Integer arguments", name)
	}
	start := defaultStart
	if len(args) == 2 {
		start = int(args[1].Int)
	}
	return args[0].Text, start, nil
}

func stringCharSearchArgs(name string, args []Value, defaultStart int) (int, int, error) {
	if len(args) != 1 && len(args) != 2 {
		return 0, 0, fmt.Errorf("%s expects Integer and optional Integer arguments", name)
	}
	if args[0].Kind != ValueInt || (len(args) == 2 && args[1].Kind != ValueInt) {
		return 0, 0, fmt.Errorf("%s expects Integer and optional Integer arguments", name)
	}
	start := defaultStart
	if len(args) == 2 {
		start = int(args[1].Int)
	}
	return int(args[0].Int), start, nil
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

func stringIndexOf(text, needle string, start int) int {
	if start < 0 {
		start = 0
	}
	if asciiPrefixLen(text, len(text)) == len(text) && asciiPrefixLen(needle, len(needle)) == len(needle) {
		if start > len(text) {
			if needle == "" {
				return len(text)
			}
			return -1
		}
		if needle == "" {
			return start
		}
		index := strings.Index(text[start:], needle)
		if index < 0 {
			return -1
		}
		return start + index
	}
	textRunes := []rune(text)
	needleRunes := []rune(needle)
	if start > len(textRunes) {
		if len(needleRunes) == 0 {
			return len(textRunes)
		}
		return -1
	}
	if len(needleRunes) == 0 {
		return start
	}
	if len(needleRunes) > len(textRunes)-start {
		return -1
	}
	for i := start; i <= len(textRunes)-len(needleRunes); i++ {
		if runesEqual(textRunes[i:i+len(needleRunes)], needleRunes) {
			return i
		}
	}
	return -1
}

func stringLastIndexOf(text, needle string, start int) int {
	textRunes := []rune(text)
	needleRunes := []rune(needle)
	if len(needleRunes) == 0 {
		if start < 0 {
			return -1
		}
		if start > len(textRunes) {
			return len(textRunes)
		}
		return start
	}
	if len(needleRunes) > len(textRunes) {
		return -1
	}
	if start > len(textRunes)-len(needleRunes) {
		start = len(textRunes) - len(needleRunes)
	}
	if start < 0 {
		return -1
	}
	for i := start; i >= 0; i-- {
		if runesEqual(textRunes[i:i+len(needleRunes)], needleRunes) {
			return i
		}
	}
	return -1
}

func stringIndexOfFold(text, needle string, start int) int {
	if start < 0 {
		start = 0
	}
	textRunes := []rune(text)
	needleRunes := []rune(needle)
	if start > len(textRunes) {
		if len(needleRunes) == 0 {
			return len(textRunes)
		}
		return -1
	}
	if len(needleRunes) == 0 {
		return start
	}
	if len(needleRunes) > len(textRunes)-start {
		return -1
	}
	for i := start; i <= len(textRunes)-len(needleRunes); i++ {
		if strings.EqualFold(string(textRunes[i:i+len(needleRunes)]), needle) {
			return i
		}
	}
	return -1
}

func stringLastIndexOfFold(text, needle string, start int) int {
	textRunes := []rune(text)
	needleRunes := []rune(needle)
	if len(needleRunes) == 0 {
		if start < 0 {
			return -1
		}
		if start > len(textRunes) {
			return len(textRunes)
		}
		return start
	}
	if len(needleRunes) > len(textRunes) {
		return -1
	}
	if start > len(textRunes)-len(needleRunes) {
		start = len(textRunes) - len(needleRunes)
	}
	if start < 0 {
		return -1
	}
	for i := start; i >= 0; i-- {
		if strings.EqualFold(string(textRunes[i:i+len(needleRunes)]), needle) {
			return i
		}
	}
	return -1
}

func stringIndexOfDifference(left, right string) int {
	leftRunes := []rune(left)
	rightRunes := []rune(right)
	limit := len(leftRunes)
	if len(rightRunes) < limit {
		limit = len(rightRunes)
	}
	for i := 0; i < limit; i++ {
		if leftRunes[i] != rightRunes[i] {
			return i
		}
	}
	if len(leftRunes) != len(rightRunes) {
		return limit
	}
	return -1
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

func stringSubstringBetween(text string, args []Value) (Value, bool, error) {
	var open, close string
	switch {
	case len(args) == 1 && args[0].Kind == ValueString:
		open = args[0].Text
		close = args[0].Text
	case len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueString:
		open = args[0].Text
		close = args[1].Text
	default:
		return Null, true, fmt.Errorf("String.substringBetween expects tag String or open and close Strings")
	}
	start := strings.Index(text, open)
	if start < 0 {
		return Null, true, nil
	}
	contentStart := start + len(open)
	end := strings.Index(text[contentStart:], close)
	if end < 0 {
		return Null, true, nil
	}
	return String(text[contentStart : contentStart+end]), true, nil
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

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)

func stripHTMLTags(text string) string {
	return html.UnescapeString(htmlTagPattern.ReplaceAllString(text, ""))
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
	pattern := args[0].Text
	if replaced, ok, err := stringRegexReplaceNegativeLookbehindLiteral(name, text, pattern, args[1].Text, all); ok || err != nil {
		return replaced, err
	}
	if stripped, lookahead, ok := stripTerminalPositiveLookahead(pattern); ok {
		return stringRegexReplaceTerminalPositiveLookahead(name, text, stripped, lookahead, args[1].Text, all)
	}
	if feature := unsupportedJavaRegexFeature(pattern); feature != "" {
		return "", unsupportedCallError(name + " " + feature)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("%s invalid regex: %w", name, err)
	}
	replacement, err := javaReplacementToGoTemplate(name, args[1].Text, re.NumSubexp())
	if err != nil {
		return "", fmt.Errorf("%s %w", name, err)
	}
	if all {
		return re.ReplaceAllString(text, replacement), nil
	}
	indices := re.FindStringSubmatchIndex(text)
	if indices == nil {
		return text, nil
	}
	var expanded []byte
	expanded = re.ExpandString(expanded, replacement, text, indices)
	return text[:indices[0]] + string(expanded) + text[indices[1]:], nil
}

func stringRegexReplaceTerminalPositiveLookahead(callee, text, pattern, lookahead, replacement string, all bool) (string, error) {
	if feature := unsupportedJavaRegexFeature(pattern); feature != "" {
		return "", unsupportedCallError(callee + " " + feature)
	}
	if feature := unsupportedJavaRegexFeature(lookahead); feature != "" {
		return "", unsupportedCallError(callee + " " + feature)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("%s invalid regex: %w", callee, err)
	}
	repl, err := javaReplacementToGoTemplate(callee, replacement, re.NumSubexp())
	if err != nil {
		return "", fmt.Errorf("%s %w", callee, err)
	}
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, nil
	}
	var out strings.Builder
	last := 0
	replaced := false
	for _, indices := range matches {
		if indices[0] < last || !regexLookaheadMatches(lookahead, text, indices[1]) {
			continue
		}
		out.WriteString(text[last:indices[0]])
		var expanded []byte
		expanded = re.ExpandString(expanded, repl, text, indices)
		out.Write(expanded)
		last = indices[1]
		replaced = true
		if !all {
			break
		}
	}
	if !replaced {
		return text, nil
	}
	out.WriteString(text[last:])
	return out.String(), nil
}

func stringRegexReplaceNegativeLookbehindLiteral(callee, text, pattern, replacement string, all bool) (string, bool, error) {
	if !strings.HasPrefix(pattern, "(?<!") {
		return "", false, nil
	}
	close := strings.IndexByte(pattern, ')')
	if close != len(pattern)-2 || close < len("(?<!)") {
		return "", false, nil
	}
	behind := pattern[len("(?<!"):close]
	if behind != `\\` {
		return "", false, nil
	}
	target := pattern[len(pattern)-1]
	repl, err := javaReplacementToGoTemplate(callee, replacement, 0)
	if err != nil {
		return "", true, fmt.Errorf("%s %w", callee, err)
	}
	var out strings.Builder
	replaced := false
	for i := 0; i < len(text); i++ {
		if text[i] == target && (i == 0 || text[i-1] != '\\') && (all || !replaced) {
			out.WriteString(repl)
			replaced = true
			continue
		}
		out.WriteByte(text[i])
	}
	if !replaced {
		return text, true, nil
	}
	return out.String(), true, nil
}

func stringRegexSplit(text string, args []Value) ([]string, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("String.split expects regex String and optional Integer limit")
	}
	if args[0].Kind != ValueString || (len(args) == 2 && args[1].Kind != ValueInt) {
		return nil, fmt.Errorf("String.split expects regex String and optional Integer limit")
	}
	limit := int64(0)
	if len(args) == 2 {
		limit = args[1].Int
	}
	return splitRegex("String.split", args[0].Text, text, limit)
}

func splitRegex(name, pattern, text string, limit int64) ([]string, error) {
	if pattern == "" {
		return splitStringCharacters(text, limit), nil
	}
	if lookahead, ok := wholePositiveLookahead(pattern); ok {
		return splitRegexPositiveLookahead(name, lookahead, text, limit)
	}
	if feature := unsupportedJavaRegexFeature(pattern); feature != "" {
		return nil, unsupportedCallError(name + " " + feature)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%s invalid regex: %w", name, err)
	}
	if splitRegexCanMatchEmpty(pattern, re) {
		return nil, unsupportedCallError(name + " regexes that can match empty strings")
	}
	if !re.MatchString(text) {
		return []string{text}, nil
	}
	if limit == 1 {
		return []string{text}, nil
	}
	if limit > 0 {
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

func splitStringCharacters(text string, limit int64) []string {
	if limit == 1 {
		return []string{text}
	}
	var parts []string
	start := 0
	splits := int64(0)
	for i := range text {
		if i == 0 {
			continue
		}
		if limit > 0 && splits >= limit-1 {
			break
		}
		parts = append(parts, text[start:i])
		start = i
		splits++
	}
	parts = append(parts, text[start:])
	if limit != 0 && (limit < 0 || splits < limit-1) {
		parts = append(parts, "")
	}
	return parts
}

func wholePositiveLookahead(pattern string) (string, bool) {
	if !strings.HasPrefix(pattern, "(?=") || !strings.HasSuffix(pattern, ")") {
		return "", false
	}
	end := regexGroupEnd(pattern, 0)
	if end != len(pattern)-1 {
		return "", false
	}
	return pattern[3:end], true
}

func splitRegexPositiveLookahead(name, lookahead, text string, limit int64) ([]string, error) {
	if feature := unsupportedJavaRegexFeature(lookahead); feature != "" {
		return nil, unsupportedCallError(name + " " + feature)
	}
	re, err := regexp.Compile("^(?:" + lookahead + ")")
	if err != nil {
		return nil, fmt.Errorf("%s invalid regex: %w", name, err)
	}
	if limit == 1 {
		return []string{text}, nil
	}
	var parts []string
	start := 0
	splits := int64(0)
	for i := nextRegexSearchIndex(text, 0); i <= len(text); i = nextRegexSearchIndex(text, i) {
		if limit > 0 && splits >= limit-1 {
			break
		}
		if !re.MatchString(text[i:]) {
			if i == len(text) {
				break
			}
			continue
		}
		parts = append(parts, text[start:i])
		start = i
		splits++
		if i == len(text) {
			break
		}
	}
	parts = append(parts, text[start:])
	if limit == 0 {
		for len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}
	return parts, nil
}

func splitRegexCanMatchEmpty(pattern string, re *regexp.Regexp) bool {
	if re.MatchString("") {
		return true
	}
	return pattern == "^" || pattern == "$" || strings.Contains(pattern, `\b`)
}

func patternSplit(source string, args []Value) ([]string, error) {
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
	}
	if args[0].Kind != ValueString || (len(args) == 2 && args[1].Kind != ValueInt) {
		return nil, fmt.Errorf("Pattern.split expects input String and optional Integer limit")
	}
	limit := int64(0)
	if len(args) == 2 {
		limit = args[1].Int
	}
	return splitRegex("Pattern.split", source, args[0].Text, limit)
}

func unsupportedJavaRegexFeature(source string) string {
	inClass := false
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
				if next == 'Q' || next == 'E' {
					return "Java regex quote escapes"
				}
				if next == 'G' {
					return "Java regex previous-match boundary"
				}
				if next == 'R' {
					return "Java regex linebreak matcher"
				}
				if next == 'X' {
					return "Java regex grapheme matcher"
				}
				if next == 'h' || next == 'H' || next == 'v' || next == 'V' {
					return "Java regex horizontal/vertical whitespace classes"
				}
				if (next == 'p' || next == 'P') && i+2 < len(source) && source[i+2] == '{' {
					end := strings.IndexByte(source[i+3:], '}')
					if end >= 0 {
						className := source[i+3 : i+3+end]
						if javaOnlyUnicodeClass(className) {
							return "Java regex Unicode character classes"
						}
					}
				}
				i++
			}
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '&':
			if inClass && i+1 < len(source) && source[i+1] == '&' {
				return "Java regex character-class intersections"
			}
		case '(':
			if inClass {
				continue
			}
			if i+2 >= len(source) || source[i+1] != '?' {
				continue
			}
			switch source[i+2] {
			case '<':
				if i+3 < len(source) && (source[i+3] == '=' || source[i+3] == '!') {
					return "Java regex lookbehind"
				}
				return "Java regex named groups"
			case 'P':
				if i+3 < len(source) && source[i+3] == '<' {
					return "Java regex named groups"
				}
			case '=', '!':
				return "Java regex lookahead"
			case '>':
				return "Java regex atomic groups"
			default:
				if unsupportedInlineJavaRegexFlags(source[i+2:]) {
					return "Java regex inline flags"
				}
			}
		case '*', '+', '?', '}':
			if !inClass && i+1 < len(source) && source[i+1] == '+' {
				return "Java regex possessive quantifiers"
			}
		}
	}
	return ""
}

func stripFixedCountPossessiveQuantifiers(source string) string {
	var out strings.Builder
	inClass := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		switch ch {
		case '\\':
			out.WriteByte(ch)
			if i+1 < len(source) {
				i++
				out.WriteByte(source[i])
			}
		case '[':
			inClass = true
			out.WriteByte(ch)
		case ']':
			inClass = false
			out.WriteByte(ch)
		case '}':
			out.WriteByte(ch)
			if !inClass && i+1 < len(source) && source[i+1] == '+' && fixedCountQuantifierEndsAt(source, i) {
				i++
			}
		default:
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func fixedCountQuantifierEndsAt(source string, end int) bool {
	start := strings.LastIndexByte(source[:end], '{')
	if start < 0 || start == end-1 {
		return false
	}
	for i := start + 1; i < end; i++ {
		if source[i] < '0' || source[i] > '9' {
			return false
		}
	}
	return true
}

func stripTerminalPositiveLookahead(source string) (string, string, bool) {
	if !strings.HasSuffix(source, ")") {
		return source, "", false
	}
	inClass := false
	depth := 0
	for i := len(source) - 1; i >= 0; i-- {
		if isEscapedRegexByte(source, i) {
			continue
		}
		switch source[i] {
		case ']':
			if depth == 0 {
				inClass = true
			}
		case '[':
			if depth == 0 {
				inClass = false
			}
		case ')':
			if !inClass {
				depth++
			}
		case '(':
			if inClass {
				continue
			}
			depth--
			if depth != 0 {
				continue
			}
			if strings.HasPrefix(source[i:], "(?=") {
				return source[:i], source[i+3 : len(source)-1], true
			}
			return source, "", false
		}
	}
	return source, "", false
}

func stripNegativeLookaheadAssertions(source string) (string, []regexNegativeLookaheadAssertion) {
	var out strings.Builder
	var lookaheads []regexNegativeLookaheadAssertion
	inClass := false
	for i := 0; i < len(source); {
		if isEscapedRegexByte(source, i) {
			out.WriteByte(source[i])
			i++
			continue
		}
		switch source[i] {
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if !inClass && strings.HasPrefix(source[i:], "(?!") {
				end := regexGroupEnd(source, i)
				if end < 0 {
					out.WriteByte(source[i])
					i++
					continue
				}
				lookaheads = append(lookaheads, regexNegativeLookaheadAssertion{
					Prefix:    out.String(),
					Lookahead: source[i+3 : end],
				})
				i = end + 1
				continue
			}
		}
		out.WriteByte(source[i])
		i++
	}
	return out.String(), lookaheads
}

func stripLeadingPositiveLookaheadAssertions(source string) (string, []string) {
	var lookaheads []string
	for strings.HasPrefix(source, "(?=") {
		end := regexGroupEnd(source, 0)
		if end < 0 {
			break
		}
		lookaheads = append(lookaheads, source[3:end])
		source = source[end+1:]
	}
	return source, lookaheads
}

func regexGroupEnd(source string, start int) int {
	if start < 0 || start >= len(source) || source[start] != '(' {
		return -1
	}
	inClass := false
	depth := 0
	for i := start; i < len(source); i++ {
		if isEscapedRegexByte(source, i) {
			continue
		}
		switch source[i] {
		case '[':
			if !inClass {
				inClass = true
			}
		case ']':
			if inClass {
				inClass = false
			}
		case '(':
			if !inClass {
				depth++
			}
		case ')':
			if !inClass {
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return -1
}

func isEscapedRegexByte(source string, index int) bool {
	slashes := 0
	for i := index - 1; i >= 0 && source[i] == '\\'; i-- {
		slashes++
	}
	return slashes%2 == 1
}

func javaRegexQuoteEscapesToGo(source string) (string, error) {
	if !strings.Contains(source, `\Q`) && !strings.Contains(source, `\E`) {
		return source, nil
	}
	var out strings.Builder
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], `\Q`) {
			i += len(`\Q`)
			end := strings.Index(source[i:], `\E`)
			if end < 0 {
				out.WriteString(regexp.QuoteMeta(source[i:]))
				return out.String(), nil
			}
			out.WriteString(regexp.QuoteMeta(source[i : i+end]))
			i += end + len(`\E`)
			continue
		}
		if strings.HasPrefix(source[i:], `\E`) {
			return "", fmt.Errorf("Java regex quote escapes")
		}
		out.WriteByte(source[i])
		i++
	}
	return out.String(), nil
}

func javaOnlyUnicodeClass(className string) bool {
	lower := strings.ToLower(className)
	return strings.HasPrefix(lower, "java") || strings.HasPrefix(className, "Is") || strings.HasPrefix(className, "In")
}

func unsupportedInlineJavaRegexFlags(suffix string) bool {
	sawFlag := false
	for i := 0; i < len(suffix); i++ {
		switch suffix[i] {
		case 'i', 'm', 's', '-':
			sawFlag = true
			continue
		case 'd', 'u', 'x', 'U':
			return true
		case ':', ')':
			return false
		default:
			return false
		}
	}
	return sawFlag
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

func escapeXML10(text string) string {
	var b strings.Builder
	for _, r := range text {
		if !validXML10Rune(r) {
			continue
		}
		if (r >= 0x7f && r <= 0x84) || (r >= 0x86 && r <= 0x9f) {
			writeNumericEntity(&b, r)
			continue
		}
		writeEscapedXMLRune(&b, r)
	}
	return b.String()
}

func escapeXML11(text string) string {
	var b strings.Builder
	for _, r := range text {
		if !validXML11Rune(r) {
			continue
		}
		if (r >= 0x1 && r <= 0x8) || (r >= 0xb && r <= 0xc) || (r >= 0xe && r <= 0x1f) || (r >= 0x7f && r <= 0x84) || (r >= 0x86 && r <= 0x9f) {
			writeNumericEntity(&b, r)
			continue
		}
		writeEscapedXMLRune(&b, r)
	}
	return b.String()
}

func validXML10Rune(r rune) bool {
	return r == 0x9 || r == 0xa || r == 0xd ||
		(r >= 0x20 && r <= 0xd7ff) ||
		(r >= 0xe000 && r <= 0xfffd) ||
		(r >= 0x10000 && r <= 0x10ffff)
}

func validXML11Rune(r rune) bool {
	return (r >= 0x1 && r <= 0xd7ff) ||
		(r >= 0xe000 && r <= 0xfffd) ||
		(r >= 0x10000 && r <= 0x10ffff)
}

func writeEscapedXMLRune(b *strings.Builder, r rune) {
	switch r {
	case '&':
		b.WriteString("&amp;")
	case '<':
		b.WriteString("&lt;")
	case '>':
		b.WriteString("&gt;")
	case '"':
		b.WriteString("&quot;")
	case '\'':
		b.WriteString("&apos;")
	default:
		b.WriteRune(r)
	}
}

func writeNumericEntity(b *strings.Builder, r rune) {
	b.WriteString("&#")
	b.WriteString(strconv.FormatInt(int64(r), 10))
	b.WriteByte(';')
}

func unescapeHTMLEntities(text string) string {
	return unescapeCoreEntities(text, false, xmlEntityAny)
}

var htmlNamedEntityReplacements = map[string]string{
	"nbsp":   "\u00a0",
	"copy":   "\u00a9",
	"reg":    "\u00ae",
	"trade":  "\u2122",
	"euro":   "\u20ac",
	"ndash":  "\u2013",
	"mdash":  "\u2014",
	"hellip": "\u2026",
	"bull":   "\u2022",
	"ldquo":  "\u201c",
	"rdquo":  "\u201d",
	"lsquo":  "\u2018",
	"rsquo":  "\u2019",
	"cent":   "\u00a2",
	"pound":  "\u00a3",
	"yen":    "\u00a5",
	"sect":   "\u00a7",
	"para":   "\u00b6",
	"middot": "\u00b7",
	"Alpha":  "Α",
	"beta":   "β",
	"Omega":  "Ω",
	"sum":    "∑",
	"rArr":   "⇒",
	"spades": "♠",
	"loz":    "◊",
}

type xmlEntityMode int

const (
	xmlEntityAny xmlEntityMode = iota
	xmlEntity10
	xmlEntity11
)

func unescapeXMLEntities(text string, mode xmlEntityMode) string {
	return unescapeCoreEntities(text, true, mode)
}

func unescapeCoreEntities(text string, xml bool, mode xmlEntityMode) string {
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
		if replacement, ok := coreEntityReplacement(entity, xml, mode); ok {
			b.WriteString(replacement)
			i = semi + 1
			continue
		}
		b.WriteString(text[i : semi+1])
		i = semi + 1
	}
	return b.String()
}

func coreEntityReplacement(entity string, xml bool, mode xmlEntityMode) (string, bool) {
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
		if r, ok := parseNumericEntity(entity[1:], mode); ok {
			return string(r), true
		}
	}
	return "", false
}

func parseNumericEntity(entity string, mode xmlEntityMode) (rune, bool) {
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
	if !validNumericEntityDigits(digits, base) {
		return 0, false
	}
	value, err := strconv.ParseInt(digits, base, 32)
	if err != nil || value <= 0 || value > utf8.MaxRune || isUTF16Surrogate(rune(value)) {
		return 0, false
	}
	r := rune(value)
	switch mode {
	case xmlEntity10:
		if !validXML10Rune(r) {
			return 0, false
		}
	case xmlEntity11:
		if !validXML11Rune(r) {
			return 0, false
		}
	}
	return r, true
}

func validNumericEntityDigits(digits string, base int) bool {
	for _, r := range digits {
		if base == 10 {
			if r < '0' || r > '9' {
				return false
			}
			continue
		}
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func isUTF16Surrogate(r rune) bool {
	return r >= 0xd800 && r <= 0xdfff
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
		case '0', '1', '2', '3', '4', '5', '6', '7':
			value := int(text[i] - '0')
			maxDigits := 2
			if text[i] > '3' {
				maxDigits = 1
			}
			for digits := 0; digits < maxDigits && i+1 < len(text); digits++ {
				next := text[i+1]
				if next < '0' || next > '7' {
					break
				}
				value = value*8 + int(next-'0')
				i++
			}
			out = append(out, rune(value))
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

func formatString(pattern string, args []Value, display func(Value) (string, error)) (string, error) {
	var out strings.Builder
	inQuote := false
	displayCache := make(map[int]string, len(args))
	for i, arg := range args {
		text, err := display(arg)
		if err != nil {
			return "", err
		}
		displayCache[i] = text
	}
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '\'':
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				out.WriteByte('\'')
				i++
				continue
			}
			inQuote = !inQuote
		case '{':
			if inQuote {
				out.WriteByte(pattern[i])
				continue
			}
			end := strings.IndexByte(pattern[i+1:], '}')
			if end < 0 {
				return "", fmt.Errorf("String.format unmatched '{' in format pattern")
			}
			token := strings.TrimSpace(pattern[i+1 : i+1+end])
			replacement, err := formatStringTokenCached(token, args, display, displayCache)
			if err != nil {
				return "", err
			}
			out.WriteString(replacement)
			i += end + 1
		case '}':
			if inQuote {
				out.WriteByte(pattern[i])
				continue
			}
			return "", fmt.Errorf("String.format unmatched '}' in format pattern")
		default:
			out.WriteByte(pattern[i])
		}
	}
	return out.String(), nil
}

func formatStringTokenCached(token string, args []Value, display func(Value) (string, error), cache map[int]string) (string, error) {
	index, ok, err := formatStringTokenIndex(token)
	if err != nil {
		return "", err
	}
	if !ok {
		return formatStringToken(token, args, display)
	}
	if index >= len(args) {
		return "{" + token + "}", nil
	}
	if cached, ok := cache[index]; ok {
		return cached, nil
	}
	replacement, err := display(args[index])
	if err != nil {
		return "", err
	}
	cache[index] = replacement
	return replacement, nil
}

func formatStringTokenIndex(token string) (int, bool, error) {
	if token == "" {
		return 0, false, fmt.Errorf("String.format empty argument index")
	}
	if strings.Contains(token, ",") {
		return 0, false, nil
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 {
		return 0, false, nil
	}
	return index, true, nil
}

func formatStringToken(token string, args []Value, display func(Value) (string, error)) (string, error) {
	if token == "" {
		return "", fmt.Errorf("String.format empty argument index")
	}
	if strings.Contains(token, ",") {
		return "", unsupportedCallError("String.format MessageFormat typed format elements")
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 {
		return "", fmt.Errorf("String.format invalid argument index %q", token)
	}
	if index >= len(args) {
		return "{" + token + "}", nil
	}
	return display(args[index])
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

func stringLevenshteinDistance(name, left string, args []Value) (int, error) {
	if len(args) != 1 && len(args) != 2 {
		return 0, fmt.Errorf("%s expects target String and optional threshold", name)
	}
	if args[0].Kind != ValueString {
		return 0, fmt.Errorf("%s expects target String", name)
	}
	if len(args) == 1 {
		return levenshteinDistance(left, args[0].Text), nil
	}
	if args[1].Kind != ValueInt {
		return 0, fmt.Errorf("%s expects Integer threshold", name)
	}
	threshold := int(args[1].Int)
	if threshold < 0 {
		return 0, fmt.Errorf("%s threshold must be non-negative", name)
	}
	return levenshteinDistanceThreshold(left, args[0].Text, threshold), nil
}

func levenshteinDistance(left, right string) int {
	return levenshteinDistanceThreshold(left, right, -1)
}

func levenshteinDistanceThreshold(left, right string, threshold int) int {
	a := []rune(left)
	b := []rune(right)
	if threshold >= 0 && absInt(len(a)-len(b)) > threshold {
		return -1
	}
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
	distance := prev[len(b)]
	if threshold >= 0 && distance > threshold {
		return -1
	}
	return distance
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
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
	start := int(args[0].Int)
	end := utf8.RuneCountInString(text)
	if len(args) == 2 {
		end = int(args[1].Int)
	}
	if start >= 0 && end >= start && asciiPrefixLen(text, end) == end {
		return String(text[start:end]), true, nil
	}
	runes := []rune(text)
	if start < 0 || start > len(runes) {
		return Null, true, newExceptionError("StringException", fmt.Sprintf("String substring index out of bounds: %d", start))
	}
	if end < 0 || end > len(runes) {
		return Null, true, newExceptionError("StringException", fmt.Sprintf("String substring index out of bounds: %d", end))
	}
	if start > end {
		return Null, true, newExceptionError("StringException", "String substring start index exceeds end index")
	}
	return String(string(runes[start:end])), true, nil
}

func asciiPrefixLen(text string, limit int) int {
	if limit > len(text) {
		limit = len(text)
	}
	for i := 0; i < limit; i++ {
		if text[i] >= utf8.RuneSelf {
			return i
		}
	}
	return limit
}

func callObjectMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalObjectMemberMethod(method)
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
	case "clone":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Object.clone expects 0 arguments")
		}
		return cloneValue(receiver), true, nil
	default:
		return Null, false, nil
	}
}

func canonicalObjectMemberMethod(method string) string {
	for _, name := range []string{"toString", "equals", "hashCode", "clone"} {
		if strings.EqualFold(method, name) {
			return name
		}
	}
	return method
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
			if typeName := typeValueText(value); typeName != "" {
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
	idText, ok := idValueText(receiver)
	if !ok {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "equals":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Bool(false), true, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Id.equals expects 1 argument")
		}
		other, ok := idValueText(args[0])
		if !ok {
			return Bool(false), true, nil
		}
		if err := validateApexID(other); err != nil {
			return Null, true, err
		}
		return Bool(apexIDTextEqual(idText, other)), true, nil
	case "tostring":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.toString expects 0 arguments")
		}
		return String(idText), true, nil
	case "to15":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.to15 expects 0 arguments")
		}
		if err := validateApexID(idText); err != nil {
			return Null, true, err
		}
		if len(idText) == 15 {
			return String(idText), true, nil
		}
		return String(idText[:15]), true, nil
	case "to18":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.to18 expects 0 arguments")
		}
		if err := validateApexID(idText); err != nil {
			return Null, true, err
		}
		if len(idText) == 18 {
			return String(idText), true, nil
		}
		return String(apexIDTo18(idText)), true, nil
	case "getsobjecttype":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Id.getSObjectType expects 0 arguments")
		}
		if err := validateApexID(idText); err != nil {
			return Null, true, err
		}
		objectName, ok := vm.sObjectNameForID(idText)
		if !ok {
			return Null, true, fmt.Errorf("System.StringException: Invalid id prefix: %s", idText[:3])
		}
		token := Object("Schema.SObjectType")
		token.Fields["object"] = String(objectName)
		return token, true, nil
	default:
		return Null, false, nil
	}
}

func idValueText(value Value) (string, bool) {
	if value.Kind == ValueString {
		return value.Text, true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		return platformScalarObjectText(value)
	}
	return "", false
}

func typedIDValueText(value Value) (string, bool) {
	if value.Kind == ValueString && strings.EqualFold(value.Type, "Id") {
		return value.Text, true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		return platformScalarObjectText(value)
	}
	return "", false
}

func displayIDText(text string) string {
	if len(text) == 15 {
		return apexIDTo18(text)
	}
	return text
}

func idMemberReceiver(value Value, method string) bool {
	switch strings.ToLower(method) {
	case "equals", "to15", "to18", "getsobjecttype":
	default:
		return false
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		return true
	}
	return value.Kind == ValueString && strings.EqualFold(value.Type, "Id")
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
	idText := ""
	if args[0].Kind == ValueString {
		idText = args[0].Text
	} else if typedID, ok := typedIDValueText(args[0]); ok {
		idText = typedID
	} else {
		return Null, newExceptionError("System.StringException", "Invalid id: "+args[0].String())
	}
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, newExceptionError("System.StringException", "Invalid id: "+idText)
		}
		if args[1].Bool {
			restored, err := restoreApexIDCasing(idText)
			if err != nil {
				return Null, newExceptionError("System.StringException", "Invalid id: "+idText)
			}
			return platformScalar("Id", restored), nil
		}
	}
	if err := validateApexID(idText); err != nil {
		return Null, newExceptionError("System.StringException", "Invalid id: "+idText)
	}
	return platformScalar("Id", idText), nil
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
