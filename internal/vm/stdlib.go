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
		blank := args[0].Kind == ValueNull || (args[0].Kind == ValueString && stringIsBlankText(args[0].Text))
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
		if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "Blob") {
			raw := ""
			if value, ok := args[0].Fields["value"]; ok && value.Kind == ValueString {
				raw = value.Text
			}
			return String(fmt.Sprintf("Blob[%d]", len(raw))), nil
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

func stringIsBlankText(text string) bool {
	text = strings.TrimSpace(text)
	return text == "" || strings.EqualFold(text, "$RecordType.Name") || strings.EqualFold(text, "$RecordType.DeveloperName")
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
	regexpSource, lookaheadSource, backreferencePairs, err := compilePatternSourceWithMetadata("Pattern.compile", args[0].Text, flags)
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
	if backreferencePairs != "" {
		pattern.Fields["backreferencePairs"] = String(backreferencePairs)
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
	regexpSource, _, _, err := compilePatternSourceWithMetadata(callee, source, flags)
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

func compilePatternSourceWithMetadata(callee, source string, flags int64) (string, string, string, error) {
	if flags < 0 {
		return "", "", "", unsupportedCallError(callee + " negative regex flags")
	}
	if unsupported := flags &^ patternSupportedFlags; unsupported != 0 {
		return "", "", "", unsupportedCallError(callee + " " + unsupportedPatternFlagsFeature(unsupported))
	}
	regexpSource := source
	lookaheadSource := ""
	backreferencePairs := ""
	if flags&patternFlagLiteral != 0 {
		regexpSource = regexp.QuoteMeta(source)
	} else {
		converted, err := javaRegexQuoteEscapesToGo(source)
		if err != nil {
			return "", "", "", unsupportedCallError(callee + " " + err.Error())
		}
		regexpSource = converted
		regexpSource, backreferencePairs = rewriteJavaNumericBackreferences(regexpSource)
		regexpSource = stripFixedCountPossessiveQuantifiers(regexpSource)
		if stripped, lookahead, ok := stripTerminalPositiveLookahead(regexpSource); ok {
			regexpSource = stripped
			lookaheadSource = stripFixedCountPossessiveQuantifiers(lookahead)
		}
		if feature := unsupportedJavaRegexFeature(regexpSource); feature != "" {
			return "", "", "", unsupportedCallError(callee + " " + feature)
		}
		if lookaheadSource != "" {
			if feature := unsupportedJavaRegexFeature(lookaheadSource); feature != "" {
				return "", "", "", unsupportedCallError(callee + " " + feature)
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
	return regexpSource, lookaheadSource, backreferencePairs, nil
}

type regexBackreferencePair struct {
	group      int
	matchGroup int
}

func rewriteJavaNumericBackreferences(source string) (string, string) {
	var out strings.Builder
	inClass := false
	groupCount := 0
	var pairs []regexBackreferencePair
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch == '\\' {
			if i+1 < len(source) {
				next := source[i+1]
				if !inClass && next >= '1' && next <= '9' {
					groupCount++
					pairs = append(pairs, regexBackreferencePair{group: int(next - '0'), matchGroup: groupCount})
					out.WriteString("(.+?)")
					i++
					continue
				}
				out.WriteByte(ch)
				out.WriteByte(next)
				i++
				continue
			}
			out.WriteByte(ch)
			continue
		}
		switch ch {
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if !inClass && regexGroupIsCapturing(source, i) {
				groupCount++
			}
		}
		out.WriteByte(ch)
	}
	return out.String(), encodeRegexBackreferencePairs(pairs)
}

func regexGroupIsCapturing(source string, index int) bool {
	if index+1 >= len(source) || source[index+1] != '?' {
		return true
	}
	if index+2 >= len(source) {
		return false
	}
	switch source[index+2] {
	case ':', '=', '!', '>', '<', 'P':
		return false
	default:
		return false
	}
}

func encodeRegexBackreferencePairs(pairs []regexBackreferencePair) string {
	if len(pairs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, strconv.Itoa(pair.group)+":"+strconv.Itoa(pair.matchGroup))
	}
	return strings.Join(parts, ",")
}

func decodeRegexBackreferencePairs(encoded string) []regexBackreferencePair {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	rawPairs := strings.Split(encoded, ",")
	pairs := make([]regexBackreferencePair, 0, len(rawPairs))
	for _, rawPair := range rawPairs {
		left, right, ok := strings.Cut(rawPair, ":")
		if !ok {
			continue
		}
		group, groupErr := strconv.Atoi(left)
		matchGroup, matchErr := strconv.Atoi(right)
		if groupErr != nil || matchErr != nil || group < 0 || matchGroup < 0 {
			continue
		}
		pairs = append(pairs, regexBackreferencePair{group: group, matchGroup: matchGroup})
	}
	return pairs
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
	backreferences := matcherBackreferencePairs(matcher)
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
			if !regexBackreferencesMatch(input, indices, backreferences) {
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
		if !regexBackreferencesMatch(input, indices, backreferences) {
			continue
		}
		return indices, nil
	}
	return nil, nil
}

func matcherFindIndices(matcher Value, re *regexp.Regexp, input string, region matcherRegionBounds, startByte int) ([]int, error) {
	lookaheadSource := matcherLookaheadSource(matcher)
	backreferences := matcherBackreferencePairs(matcher)
	if lookaheadSource != "" {
		return matcherFindIndicesWithTerminalLookahead(matcher, input, region, startByte)
	}
	if !matcherUsesFullInputBounds(matcher) {
		if len(backreferences) > 0 {
			searchStart := startByte
			if searchStart < region.startByte {
				searchStart = region.startByte
			}
			for _, indices := range re.FindAllStringSubmatchIndex(input[searchStart:region.endByte], -1) {
				if indices == nil {
					continue
				}
				offsetRegexIndices(indices, searchStart)
				if regexBackreferencesMatch(input, indices, backreferences) {
					return indices, nil
				}
			}
			return nil, nil
		}
		indices := re.FindStringSubmatchIndex(input[startByte:region.endByte])
		if indices != nil {
			offsetRegexIndices(indices, startByte)
			if !regexBackreferencesMatch(input, indices, backreferences) {
				return nil, nil
			}
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
			if !regexBackreferencesMatch(input, indices, backreferences) {
				continue
			}
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

func matcherReplaceWithMetadata(matcher Value, name string, re *regexp.Regexp, input string, region matcherRegionBounds, args []Value, all bool) (string, error) {
	backreferences := matcherBackreferencePairs(matcher)
	if len(backreferences) == 0 {
		return matcherReplace(name, re, input, region, args, all)
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return "", fmt.Errorf("%s expects replacement String", name)
	}
	replacement, err := javaReplacementToGoTemplate(name, args[0].Text, re.NumSubexp())
	if err != nil {
		return "", fmt.Errorf("%s %w", name, err)
	}
	regionText := input[region.startByte:region.endByte]
	var out strings.Builder
	last := 0
	replaced := false
	for _, indices := range re.FindAllStringSubmatchIndex(regionText, -1) {
		if len(indices) < 2 || indices[0] < last {
			continue
		}
		if !regexBackreferencesMatch(regionText, indices, backreferences) {
			continue
		}
		out.WriteString(regionText[last:indices[0]])
		var expanded []byte
		expanded = re.ExpandString(expanded, replacement, regionText, indices)
		out.Write(expanded)
		last = indices[1]
		replaced = true
		if !all {
			break
		}
	}
	if !replaced {
		return input, nil
	}
	out.WriteString(regionText[last:])
	return input[:region.startByte] + out.String() + input[region.endByte:], nil
}

func matcherBackreferencePairs(matcher Value) []regexBackreferencePair {
	value, ok := matcher.Fields["backreferencePairs"]
	if !ok || value.Kind != ValueString {
		return nil
	}
	return decodeRegexBackreferencePairs(value.Text)
}

func regexBackreferencesMatch(input string, indices []int, pairs []regexBackreferencePair) bool {
	for _, pair := range pairs {
		groupStart := pair.group * 2
		matchStart := pair.matchGroup * 2
		if groupStart+1 >= len(indices) || matchStart+1 >= len(indices) {
			return false
		}
		leftStart, leftEnd := indices[groupStart], indices[groupStart+1]
		rightStart, rightEnd := indices[matchStart], indices[matchStart+1]
		if leftStart < 0 || leftEnd < 0 || rightStart < 0 || rightEnd < 0 {
			return false
		}
		if input[leftStart:leftEnd] != input[rightStart:rightEnd] {
			return false
		}
	}
	return true
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
		if item.Kind != ValueObject {
			continue
		}
		for _, typeName := range []string{item.Type, item.Runtime, item.Static} {
			if typeName == "" || strings.EqualFold(typeName, "SObject") || strings.EqualFold(typeName, "Object") {
				continue
			}
			if isCommonSObjectTypeName(typeName) || strings.HasSuffix(typeName, "__c") || strings.HasSuffix(typeName, "__e") || strings.HasSuffix(typeName, "__mdt") {
				return typeName
			}
		}
		if objectName, ok := item.Fields["object"]; ok && objectName.Kind == ValueString && objectName.Text != "" {
			return objectName.Text
		}
	}
	return ""
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
	if replaced, ok, err := stringRegexReplaceQuotedPositiveLookaround(name, text, pattern, args[1].Text, all); ok || err != nil {
		return replaced, err
	}
	if stripped, lookahead, ok := stripTerminalPositiveLookahead(pattern); ok {
		return stringRegexReplaceTerminalPositiveLookahead(name, text, stripped, lookahead, args[1].Text, all)
	}
	converted, err := javaRegexQuoteEscapesToGo(pattern)
	if err != nil {
		return "", unsupportedCallError(name + " " + err.Error())
	}
	pattern = converted
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

func stringRegexReplaceQuotedPositiveLookaround(callee, text, pattern, replacement string, all bool) (string, bool, error) {
	if !strings.HasPrefix(pattern, `(?<=")`) || !strings.HasSuffix(pattern, `(?=")`) {
		return "", false, nil
	}
	core := pattern[len(`(?<=")`) : len(pattern)-len(`(?=")`)]
	if feature := unsupportedJavaRegexFeature(core); feature != "" {
		return "", true, unsupportedCallError(callee + " " + feature)
	}
	re, err := regexp.Compile(`"` + core + `"`)
	if err != nil {
		return "", true, fmt.Errorf("%s invalid regex: %w", callee, err)
	}
	repl, err := javaReplacementToGoTemplate(callee, `"`+replacement+`"`, re.NumSubexp())
	if err != nil {
		return "", true, fmt.Errorf("%s %w", callee, err)
	}
	if all {
		return re.ReplaceAllString(text, repl), true, nil
	}
	indices := re.FindStringSubmatchIndex(text)
	if indices == nil {
		return text, true, nil
	}
	var expanded []byte
	expanded = re.ExpandString(expanded, repl, text, indices)
	return text[:indices[0]] + string(expanded) + text[indices[1]:], true, nil
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
		if validateApexID(other) != nil {
			return Bool(false), true, nil
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
