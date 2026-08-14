package vm

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

func parseLiteral(raw string) (Value, error) {
	original := raw
	switch strings.ToLower(raw) {
	case "true":
		return Bool(true), nil
	case "false":
		return Bool(false), nil
	case "null":
		return Null, nil
	}
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") {
		text := strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'")
		return String(unescapeApexStringLiteral(text)), nil
	}
	suffix := byte(0)
	if len(raw) > 0 {
		last := raw[len(raw)-1]
		if strings.ContainsRune("LlDdFf", rune(last)) {
			suffix = last
			raw = raw[:len(raw)-1]
		}
	}
	isLong := suffix == 'L' || suffix == 'l'
	isDecimal := suffix == 'D' || suffix == 'd' || suffix == 'F' || suffix == 'f' || strings.ContainsAny(raw, ".eE")
	if isDecimal {
		value, err := decimalFromText(raw)
		if err != nil {
			return Null, fmt.Errorf("invalid decimal literal %q", original)
		}
		return value, nil
	}
	numberRaw := raw
	value, err := strconv.ParseInt(numberRaw, 10, 64)
	if err != nil {
		return Null, fmt.Errorf("invalid literal %q", raw)
	}
	out := Int(value)
	if isLong {
		out.Type = "Long"
	}
	return out, nil
}

func unescapeApexStringLiteral(text string) string {
	if !strings.ContainsAny(text, "'\\") {
		return text
	}
	var out strings.Builder
	out.Grow(len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '\'' && i+1 < len(text) && text[i+1] == '\'' {
			out.WriteByte('\'')
			i++
			continue
		}
		if text[i] != '\\' || i+1 >= len(text) {
			out.WriteByte(text[i])
			continue
		}
		i++
		switch text[i] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case '\\':
			out.WriteByte('\\')
		case '\'':
			out.WriteByte('\'')
		case '"':
			out.WriteByte('"')
		default:
			out.WriteByte('\\')
			out.WriteByte(text[i])
		}
	}
	return out.String()
}

func evalUnary(op string, value Value) (Value, error) {
	switch op {
	case "!":
		boolValue, ok := booleanOperand(value)
		if !ok {
			return Null, fmt.Errorf("operator ! requires Boolean, got %s", value.Kind)
		}
		return Bool(!boolValue), nil
	case "-":
		if value.Kind == ValueDecimal {
			if isFloatBackedDecimal(value) {
				return decimalAsDouble(Decimal(-value.Decimal)), nil
			}
			if rat, ok := valueDecimalRat(value); ok {
				return decimalFromRat(new(big.Rat).Neg(rat), int64(decimalScale(value))), nil
			}
			return Decimal(-value.Decimal), nil
		}
		if value.Kind != ValueInt {
			return Null, fmt.Errorf("operator - requires numeric value, got %s", value.Kind)
		}
		if value.Int == math.MinInt64 {
			return Null, fmt.Errorf("integer unary - overflow")
		}
		result := Int(-value.Int)
		if isLongIntValue(value) {
			result.Type = "Long"
		}
		return result, nil
	case "+":
		return value, nil
	default:
		return Null, fmt.Errorf("unsupported unary operator %q", op)
	}
}

func evalBinary(op string, left, right Value) (Value, error) {
	switch op {
	case "+":
		if value, ok, err := platformDateArithmetic("+", left, right); ok || err != nil {
			return value, err
		}
		if isStringConcatOperand(left) || isStringConcatOperand(right) {
			return String(concatStringText(left) + concatStringText(right)), nil
		}
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a + b })
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a + b })
	case "-":
		if value, ok, err := platformDateArithmetic("-", left, right); ok || err != nil {
			return value, err
		}
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a - b })
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a - b })
	case "*":
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a * b })
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a * b })
	case "/":
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			if decimalOf(right) == 0 {
				return Null, newExceptionError("MathException", "Divide by 0")
			}
			if !isFloatBackedDecimal(left) && !isFloatBackedDecimal(right) {
				return Null, unsupportedCallError("Decimal division exact semantics are deferred")
			}
			return decimalBinary(op, left, right, func(a, b float64) float64 { return a / b })
		}
		if right.Kind == ValueInt && right.Int == 0 {
			return Null, newExceptionError("MathException", "Divide by 0")
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a / b })
	case "%":
		if right.Kind == ValueInt && right.Int == 0 {
			return Null, newExceptionError("MathException", "Divide by 0")
		}
		return intBinary(op, left, right, func(a, b int64) int64 { return a % b })
	case "<<", ">>":
		if left.Kind != ValueInt || right.Kind != ValueInt {
			return Null, fmt.Errorf("operator %s requires Integer operands", op)
		}
		if right.Int < 0 || right.Int > 63 {
			return Null, fmt.Errorf("operator %s shift count out of range", op)
		}
		if op == "<<" {
			result := Int(left.Int << uint(right.Int))
			if isLongIntValue(left) {
				result.Type = "Long"
			}
			return result, nil
		}
		result := Int(left.Int >> uint(right.Int))
		if isLongIntValue(left) {
			result.Type = "Long"
		}
		return result, nil
	case "==":
		return Bool(left.Equal(right)), nil
	case "!=":
		return Bool(!left.Equal(right)), nil
	case "<", "<=", ">", ">=":
		if left.Kind == ValueNull || right.Kind == ValueNull {
			return Bool(false), nil
		}
		if left.Kind == ValueString && right.Kind == ValueString {
			switch op {
			case "<":
				return Bool(left.Text < right.Text), nil
			case "<=":
				return Bool(left.Text <= right.Text), nil
			case ">":
				return Bool(left.Text > right.Text), nil
			default:
				return Bool(left.Text >= right.Text), nil
			}
		}
		if comparable, ok := comparablePlatformTemporal(left, right); ok {
			a, b := comparable[0], comparable[1]
			switch op {
			case "<":
				return Bool(a.Before(b)), nil
			case "<=":
				return Bool(!a.After(b)), nil
			case ">":
				return Bool(a.After(b)), nil
			default:
				return Bool(!a.Before(b)), nil
			}
		}
		if comparable, ok := comparablePlatformScalarText(left, right); ok {
			a, b := comparable[0], comparable[1]
			switch op {
			case "<":
				return Bool(a < b), nil
			case "<=":
				return Bool(a <= b), nil
			case ">":
				return Bool(a > b), nil
			default:
				return Bool(a >= b), nil
			}
		}
		if !isNumeric(left) || !isNumeric(right) {
			return Null, fmt.Errorf("operator %s requires numeric operands", op)
		}
		if left.Kind == ValueDecimal || right.Kind == ValueDecimal {
			aRat, aOK := valueDecimalRat(left)
			bRat, bOK := valueDecimalRat(right)
			if aOK && bOK {
				comparison := aRat.Cmp(bRat)
				switch op {
				case "<":
					return Bool(comparison < 0), nil
				case "<=":
					return Bool(comparison <= 0), nil
				case ">":
					return Bool(comparison > 0), nil
				default:
					return Bool(comparison >= 0), nil
				}
			}
			a, b := decimalOf(left), decimalOf(right)
			switch op {
			case "<":
				return Bool(a < b), nil
			case "<=":
				return Bool(a <= b), nil
			case ">":
				return Bool(a > b), nil
			default:
				return Bool(a >= b), nil
			}
		}
		switch op {
		case "<":
			return Bool(left.Int < right.Int), nil
		case "<=":
			return Bool(left.Int <= right.Int), nil
		case ">":
			return Bool(left.Int > right.Int), nil
		default:
			return Bool(left.Int >= right.Int), nil
		}
	case "&&", "||":
		leftBool, leftOK := booleanOperand(left)
		rightBool, rightOK := booleanOperand(right)
		if !leftOK || !rightOK {
			return Null, fmt.Errorf("operator %s requires Boolean operands", op)
		}
		if op == "&&" {
			return Bool(leftBool && rightBool), nil
		}
		return Bool(leftBool || rightBool), nil
	case "&", "|":
		leftBool, leftOK := booleanOperand(left)
		rightBool, rightOK := booleanOperand(right)
		if !leftOK || !rightOK {
			return Null, fmt.Errorf("operator %s requires Boolean operands", op)
		}
		if op == "&" {
			return Bool(leftBool && rightBool), nil
		}
		return Bool(leftBool || rightBool), nil
	default:
		return Null, fmt.Errorf("unsupported binary operator %q", op)
	}
}

func platformDateArithmetic(op string, left, right Value) (Value, bool, error) {
	if (op != "+" && op != "-") || left.Kind != ValueObject {
		return Null, false, nil
	}
	days, ok := numericDaysOperand(right)
	if !ok {
		return Null, false, nil
	}
	if op == "-" {
		days = -days
	}
	switch left.Type {
	case "Date":
		date, err := parsePlatformDate(left)
		if err != nil {
			return Null, true, err
		}
		return platformScalar("Date", date.Add(platformDayDuration(days)).Format("2006-01-02")), true, nil
	case "Datetime":
		datetime, err := parsePlatformDatetime(left)
		if err != nil {
			return Null, true, err
		}
		return platformScalar("Datetime", formatPlatformDatetime(datetime.Add(platformDayDuration(days)))), true, nil
	default:
		return Null, false, nil
	}
}

func numericDaysOperand(value Value) (float64, bool) {
	switch value.Kind {
	case ValueInt:
		return float64(value.Int), true
	case ValueDecimal:
		return value.Decimal, true
	default:
		return 0, false
	}
}

func platformDayDuration(days float64) time.Duration {
	milliseconds := math.Round(days * 24 * 60 * 60 * 1000)
	return time.Duration(milliseconds) * time.Millisecond
}

func booleanOperand(value Value) (bool, bool) {
	if value.Kind == ValueBool {
		return value.Bool, true
	}
	if value.Kind == ValueNull && strings.EqualFold(value.Type, "Boolean") {
		return false, true
	}
	return false, false
}

func isStringConcatOperand(value Value) bool {
	if value.Kind == ValueString || (value.Kind == ValueNull && strings.EqualFold(canonicalApexScalarType(value.Type), "String")) {
		return true
	}
	return value.Kind == ValueObject && strings.EqualFold(value.Type, "Schema.SObjectField")
}

func concatStringText(value Value) string {
	if value.Kind == ValueDecimal {
		return decimalDisplayText(value)
	}
	return value.String()
}

func comparablePlatformScalarText(left, right Value) ([2]string, bool) {
	if leftText, leftOK := comparableIDText(left); leftOK {
		if rightText, rightOK := comparableIDText(right); rightOK {
			return [2]string{leftText, rightText}, true
		}
	}
	if left.Kind != ValueObject || right.Kind != ValueObject || !strings.EqualFold(left.Type, right.Type) {
		return [2]string{}, false
	}
	switch strings.ToLower(left.Type) {
	case "date", "datetime", "time", "id":
	default:
		return [2]string{}, false
	}
	leftText, err := platformScalarText(left, left.Type)
	if err != nil || leftText == "" {
		return [2]string{}, false
	}
	rightText, err := platformScalarText(right, right.Type)
	if err != nil || rightText == "" {
		return [2]string{}, false
	}
	return [2]string{leftText, rightText}, true
}

func comparablePlatformTemporal(left, right Value) ([2]time.Time, bool) {
	leftTime, leftOK := comparablePlatformTemporalValue(left)
	rightTime, rightOK := comparablePlatformTemporalValue(right)
	if !leftOK || !rightOK {
		return [2]time.Time{}, false
	}
	return [2]time.Time{leftTime, rightTime}, true
}

func comparablePlatformTemporalValue(value Value) (time.Time, bool) {
	if value.Kind == ValueString {
		parsed, err := parseDatetimeTextAllowDateOnly(value.Text)
		return parsed.UTC(), err == nil
	}
	if value.Kind != ValueObject {
		return time.Time{}, false
	}
	switch strings.ToLower(value.Type) {
	case "date":
		parsed, err := parsePlatformDate(value)
		return parsed.UTC(), err == nil
	case "datetime", "dateTime":
		parsed, err := parsePlatformDatetime(value)
		return parsed.UTC(), err == nil
	default:
		return time.Time{}, false
	}
}

func comparableIDText(value Value) (string, bool) {
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		text, err := platformScalarText(value, value.Type)
		return text, err == nil && text != ""
	}
	if value.Kind != ValueString {
		return "", false
	}
	if strings.EqualFold(value.Type, "Id") {
		return value.Text, value.Text != ""
	}
	if err := validateApexIDShape(value.Text); err == nil {
		return value.Text, true
	}
	return "", false
}

func intBinary(op string, left, right Value, fn func(int64, int64) int64) (Value, error) {
	if left.Kind == ValueNull && right.Kind == ValueInt {
		left = Int(0)
	}
	if right.Kind == ValueNull && left.Kind == ValueInt {
		right = Int(0)
	}
	if left.Kind != ValueInt || right.Kind != ValueInt {
		return Null, fmt.Errorf("operator %s requires Integer operands", op)
	}
	if err := checkIntBinaryOverflow(op, left.Int, right.Int); err != nil {
		return Null, err
	}
	result := Int(fn(left.Int, right.Int))
	if isLongIntValue(left) || isLongIntValue(right) {
		result.Type = "Long"
	}
	return result, nil
}

func decimalBinary(op string, left, right Value, fn func(float64, float64) float64) (Value, error) {
	if left.Kind == ValueNull && isNumeric(right) {
		left = Decimal(0)
	}
	if right.Kind == ValueNull && isNumeric(left) {
		right = Decimal(0)
	}
	if !isNumeric(left) || !isNumeric(right) {
		return Null, fmt.Errorf("operator %s requires numeric operands", op)
	}
	if op == "/" && !isFloatBackedDecimal(left) && !isFloatBackedDecimal(right) {
		return Null, unsupportedCallError("Decimal division exact semantics are deferred")
	}
	result, ok := preciseDecimalBinary(op, left, right)
	if !ok {
		floatResult := fn(decimalOf(left), decimalOf(right))
		if math.IsInf(floatResult, 0) || math.IsNaN(floatResult) {
			return Null, fmt.Errorf("operator %s result must be finite", op)
		}
		result := Decimal(floatResult)
		if isFloatBackedDecimal(left) || isFloatBackedDecimal(right) {
			result.Static = "Double"
		}
		return result, nil
	}
	return result, nil
}

func preciseDecimalBinary(op string, left, right Value) (Value, bool) {
	leftRat, ok := valueDecimalRat(left)
	if !ok {
		return Null, false
	}
	rightRat, ok := valueDecimalRat(right)
	if !ok {
		return Null, false
	}
	result := new(big.Rat)
	scale := 0
	switch op {
	case "+":
		result.Add(leftRat, rightRat)
		scale = max(decimalScale(left), decimalScale(right))
	case "-":
		result.Sub(leftRat, rightRat)
		scale = max(decimalScale(left), decimalScale(right))
	case "*":
		result.Mul(leftRat, rightRat)
		scale = decimalScale(left) + decimalScale(right)
	case "/":
		if rightRat.Sign() == 0 {
			return Null, false
		}
		return Null, false
	default:
		return Null, false
	}
	out := decimalFromRat(result, int64(scale))
	if math.IsInf(out.Decimal, 0) || math.IsNaN(out.Decimal) {
		return Null, false
	}
	out.Text = normalizeComputedDecimalText(out.Text)
	return out, true
}

func normalizeComputedDecimalText(text string) string {
	text = strings.TrimSpace(text)
	if strings.Contains(text, ".") {
		text = strings.TrimRight(text, "0")
		text = strings.TrimRight(text, ".")
	}
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func checkIntBinaryOverflow(op string, left, right int64) error {
	switch op {
	case "+":
		if (right > 0 && left > math.MaxInt64-right) || (right < 0 && left < math.MinInt64-right) {
			return fmt.Errorf("operator + integer overflow")
		}
	case "-":
		if (right < 0 && left > math.MaxInt64+right) || (right > 0 && left < math.MinInt64+right) {
			return fmt.Errorf("operator - integer overflow")
		}
	case "*":
		if left != 0 && right != 0 {
			if left == math.MinInt64 && right == -1 || right == math.MinInt64 && left == -1 {
				return fmt.Errorf("operator * integer overflow")
			}
			product := left * right
			if product/right != left {
				return fmt.Errorf("operator * integer overflow")
			}
		}
	case "/":
		if left == math.MinInt64 && right == -1 {
			return fmt.Errorf("operator / integer overflow")
		}
	case "%":
		if left == math.MinInt64 && right == -1 {
			return fmt.Errorf("operator %% integer overflow")
		}
	}
	return nil
}

func isNumeric(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueDecimal
}

func decimalOf(value Value) float64 {
	if value.Kind == ValueDecimal {
		return value.Decimal
	}
	return float64(value.Int)
}

func coerceAssignable(typeName string, value Value) (Value, error) {
	typeName = canonicalRuntimePlatformType(typeName)
	value.Type = canonicalRuntimePlatformType(value.Type)
	if value.Kind == ValueNull {
		value.Type = typeName
		return value, nil
	}
	canonicalType := canonicalApexScalarType(typeName)
	switch canonicalType {
	case "Integer", "Long":
		if value.Kind == ValueInt {
			return integerValueForType(value, canonicalType), nil
		}
	case "Decimal", "Double":
		if value.Kind == ValueInt {
			decimal, err := decimalFromText(strconv.FormatInt(value.Int, 10))
			if err != nil {
				return Null, err
			}
			if strings.EqualFold(canonicalType, "Double") {
				return decimalAsDouble(decimal), nil
			}
			return decimal, nil
		}
		if value.Kind == ValueDecimal {
			if strings.EqualFold(canonicalType, "Double") {
				return decimalAsDouble(value), nil
			}
			if isFloatBackedDecimal(value) {
				return Decimal(value.Decimal), nil
			}
			return value, nil
		}
	case "Boolean":
		if value.Kind == ValueBool {
			return value, nil
		}
	case "String":
		if value.Kind == ValueString {
			return value, nil
		}
		if idText, ok := typedIDValueText(value); ok {
			return String(displayIDText(idText)), nil
		}
		if text, ok := platformScalarObjectText(value); ok {
			return String(text), nil
		}
	case "Id":
		if value.Kind == ValueString {
			return value, nil
		}
	case "Object":
		return value, nil
	}
	if value.Kind == ValueObject {
		if strings.EqualFold(typeName, value.Type) || platformTokenTypeAlias(value.Type, typeName) {
			return value, nil
		}
		return Null, fmt.Errorf("cannot assign %s to %s", value.Type, typeName)
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueList {
		return coerceCollectionValue(typeName, value)
	}
	if collectionBase(typeName) == "List" && value.Kind == ValueMap {
		if records, ok := queryResultRecordsList(value); ok {
			return coerceCollectionValue(typeName, records)
		}
	}
	if collectionBase(typeName) == "Set" && value.Kind == ValueSet {
		return coerceCollectionValue(typeName, value)
	}
	if isMapType(typeName) && value.Kind == ValueMap {
		return coerceCollectionValue(typeName, value)
	}
	return Null, fmt.Errorf("cannot assign %s to %s", value.Kind, typeName)
}

func canonicalApexScalarType(typeName string) string {
	typeName = canonicalRuntimePlatformType(typeName)
	switch {
	case strings.EqualFold(typeName, "Integer"):
		return "Integer"
	case strings.EqualFold(typeName, "Long"):
		return "Long"
	case strings.EqualFold(typeName, "Decimal"):
		return "Decimal"
	case strings.EqualFold(typeName, "Double"):
		return "Double"
	case strings.EqualFold(typeName, "Boolean"):
		return "Boolean"
	case strings.EqualFold(typeName, "String"):
		return "String"
	case strings.EqualFold(typeName, "Id"):
		return "Id"
	case strings.EqualFold(typeName, "Object"):
		return "Object"
	default:
		return typeName
	}
}

func canonicalRuntimePlatformType(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	if keyType, valueType, ok := mapTypeArgs(typeName); ok {
		return "Map<" + canonicalRuntimePlatformType(keyType) + "," + canonicalRuntimePlatformType(valueType) + ">"
	}
	if base := collectionBase(typeName); base != "" {
		if elementType, ok := collectionElementType(typeName); ok {
			return base + "<" + canonicalRuntimePlatformType(elementType) + ">"
		}
		return base
	}
	if rest, ok := stripLeadingSystemNamespace(typeName); ok {
		switch strings.ToLower(rest) {
		case "type", "object", "list", "set", "map", "iterable", "iterator", "stubprovider", "callable", "httpcalloutmock", "statuscode":
			return rest
		}
	}
	return typeName
}

func ensureAssignable(typeName string, value Value) error {
	_, err := coerceAssignable(typeName, value)
	return err
}

func coerceCollectionValue(typeName string, value Value) (Value, error) {
	if value.Type != "" && !strings.EqualFold(value.Type, typeName) && value.Runtime == "" {
		value.Runtime = value.Type
	}
	value.Type = typeName
	switch value.Kind {
	case ValueList:
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		if strings.EqualFold(elementType, "SObject") && len(value.List) == 0 {
			if runtimeElementType, ok := collectionElementType(value.Runtime); ok && !collectionElementCarriesSObjectType(runtimeElementType) {
				value.Runtime = ""
			}
			value.Static = typeName
			return value, nil
		}
		for i, item := range value.List {
			coerced, err := coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			value.List[i] = coerced
		}
	case ValueSet:
		elementType, ok := collectionElementType(typeName)
		if !ok {
			return value, nil
		}
		out := make([]Value, 0, len(value.Set))
		for _, item := range value.Set {
			coerced, err := coerceAssignable(elementType, item)
			if err != nil {
				return Null, err
			}
			if !containsValue(out, coerced) {
				out = append(out, coerced)
			}
		}
		value.Set = out
	}
	return value, nil
}

func collectionElementCarriesSObjectType(typeName string) bool {
	typeName = canonicalRuntimePlatformType(typeName)
	if prefix, rest, ok := strings.Cut(strings.TrimSpace(typeName), "."); ok && strings.EqualFold(prefix, "Schema") {
		typeName = rest
	}
	if strings.Contains(typeName, ".") {
		typeName = shortTypeName(typeName)
	}
	if strings.EqualFold(typeName, "SObject") {
		return true
	}
	if strings.EqualFold(typeName, "AggregateResult") {
		return false
	}
	return isCommonSObjectTypeName(typeName) || isCustomObjectLikeName(typeName)
}

func constructValue(typeName string, args []Value) (Value, error) {
	switch {
	case collectionBase(typeName) == "List":
		value := List(args...)
		return coerceCollectionValue(typeName, value)
	case collectionBase(typeName) == "Set":
		value := Set(args...)
		return coerceCollectionValue(typeName, value)
	case isMapType(typeName):
		if len(args) != 0 {
			return Null, fmt.Errorf("Map constructor does not accept positional values")
		}
		value := Map()
		value.Type = typeName
		return value, nil
	default:
		if len(args) != 0 {
			return Null, fmt.Errorf("%s constructor does not accept arguments", typeName)
		}
		return Object(typeName), nil
	}
}

func collectionElementType(typeName string) (string, bool) {
	typeName = strings.TrimSpace(typeName)
	if strings.HasSuffix(typeName, "[]") {
		element := strings.TrimSpace(strings.TrimSuffix(typeName, "[]"))
		return element, element != ""
	}
	args, ok := genericTypeArgs(typeName)
	if !ok || len(args) != 1 {
		return "", false
	}
	return args[0], true
}

func mapTypeArgs(typeName string) (string, string, bool) {
	args, ok := genericTypeArgs(typeName)
	if !ok || len(args) != 2 {
		return "", "", false
	}
	return args[0], args[1], true
}

func genericTypeArgs(typeName string) ([]string, bool) {
	open := strings.IndexByte(typeName, '<')
	if open < 0 || !strings.HasSuffix(typeName, ">") {
		return nil, false
	}
	inner := typeName[open+1 : len(typeName)-1]
	var args []string
	depth := 0
	start := 0
	for i, r := range inner {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[start:]))
	for _, arg := range args {
		if arg == "" {
			return nil, false
		}
	}
	return args, true
}
