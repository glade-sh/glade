package vm

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

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
		if method == "longValue" {
			return longIntValue(receiver.Int), receiver, false, true, nil
		}
		return Int(receiver.Int), receiver, false, true, nil
	case "decimalValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Integer.%s expects 0 arguments", method)
		}
		value, err := decimalFromText(strconv.FormatInt(receiver.Int, 10))
		return value, receiver, false, true, err
	default:
		return Null, receiver, false, false, nil
	}
}
func callDecimalMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if isFloatBackedDecimal(receiver) && isDoubleUnsupportedMember(method) {
		return Null, receiver, false, true, unsupportedCallError("Double." + method)
	}
	switch method {
	case "abs":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.abs expects 0 arguments")
		}
		value, err := decimalAbsValue(receiver)
		return value, receiver, false, true, err
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
		value, err := roundDecimalValueToScale("Decimal.setScale", receiver, args[0].Int, mode)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return value, receiver, false, true, nil
	case "round":
		if len(args) > 1 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.round expects optional RoundingMode")
		}
		mode := "HALF_EVEN"
		if len(args) == 0 && strings.EqualFold(receiver.Static, "Double") {
			mode = "HALF_UP"
		}
		if len(args) == 1 {
			parsedMode, err := decimalRoundingMode(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			mode = parsedMode
		}
		roundedDecimal, err := roundDecimalValueToScale("Decimal.round", receiver, 0, mode)
		if err != nil {
			return Null, receiver, false, true, err
		}
		rounded, err := strconv.ParseInt(decimalPlainText(roundedDecimal), 10, 64)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return longIntValue(rounded), receiver, false, true, nil
	case "intValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.intValue expects 0 arguments")
		}
		converted, err := int32FromDecimalValue("Decimal.intValue", receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return Int(int64(converted)), receiver, false, true, nil
	case "longValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.longValue expects 0 arguments")
		}
		converted, err := int64FromDecimalValue("Decimal.longValue", receiver)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return longIntValue(converted), receiver, false, true, nil
	case "doubleValue":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.doubleValue expects 0 arguments")
		}
		if err := ensureFiniteDecimal("Decimal.doubleValue", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		return decimalAsDouble(receiver), receiver, false, true, nil
	case "pow":
		if len(args) != 1 || args[0].Kind != ValueInt || isLongIntValue(args[0]) {
			return Null, receiver, false, true, fmt.Errorf("Decimal.pow expects Integer")
		}
		if err := ensureFiniteDecimal("Decimal.pow", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		if rat, ok := valueDecimalRat(receiver); ok {
			if args[0].Int < 0 {
				return Null, receiver, false, true, unsupportedCallError("Decimal.pow negative exponent exact semantics are deferred")
			}
			exponent := big.NewInt(args[0].Int)
			numerator := new(big.Int).Exp(rat.Num(), exponent, nil)
			denominator := new(big.Int).Exp(rat.Denom(), exponent, nil)
			resultRat := new(big.Rat).SetFrac(numerator, denominator)
			scale, ok := terminatingDecimalScale(resultRat)
			if !ok {
				return Null, receiver, false, true, unsupportedCallError("Decimal.pow exact result cannot be represented")
			}
			result := decimalFromRat(resultRat, scale)
			if err := ensureFiniteDecimal("Decimal.pow result", result.Decimal); err != nil {
				return Null, receiver, false, true, err
			}
			result.Text = normalizeComputedDecimalText(result.Text)
			return result, receiver, false, true, nil
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
		return String(formatDecimalTextWithGrouping(decimalDisplayText(receiver))), receiver, false, true, nil
	case "toPlainString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Decimal.toPlainString expects 0 arguments")
		}
		if err := ensureFiniteDecimal("Decimal.toPlainString", receiver.Decimal); err != nil {
			return Null, receiver, false, true, err
		}
		if receiver.Text != "" {
			if strings.IndexAny(receiver.Text, "eE") >= 0 {
				rat, ok := new(big.Rat).SetString(receiver.Text)
				if ok && rat.IsInt() {
					return String(rat.Num().String()), receiver, false, true, nil
				}
				if ok {
					return String(rat.FloatString(int(decimalScale(receiver)))), receiver, false, true, nil
				}
			}
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
		if expIdx := strings.IndexAny(text, "eE"); expIdx >= 0 {
			mantissa := text[:expIdx]
			expStr := text[expIdx:]
			if dot := strings.IndexByte(mantissa, '.'); dot >= 0 {
				mantissa = strings.TrimRight(mantissa, "0")
				mantissa = strings.TrimRight(mantissa, ".")
			}
			rat, ok := new(big.Rat).SetString(mantissa + expStr)
			if ok && rat.Sign() == 0 {
				text = "0"
			} else {
				text = mantissa + expStr
			}
		} else {
			if dot := strings.IndexByte(text, '.'); dot >= 0 {
				text = strings.TrimRight(text, "0")
				text = strings.TrimRight(text, ".")
			}
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
		if args[0].Kind != ValueDecimal {
			return Null, receiver, false, true, fmt.Errorf("Decimal.divide expects Decimal divisor")
		}
		if args[1].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Decimal.divide expects Integer scale")
		}
		mode := "HALF_EVEN"
		if len(args) == 3 {
			parsedMode, err := decimalRoundingMode(args[2])
			if err != nil {
				return Null, receiver, false, true, err
			}
			mode = parsedMode
		}
		dividend, dividendOK := valueDecimalRat(receiver)
		divisor, divisorOK := valueDecimalRat(args[0])
		if !dividendOK || !divisorOK {
			return Null, receiver, false, true, unsupportedCallError("Decimal division exact semantics are deferred")
		}
		if divisor.Sign() == 0 {
			return Null, receiver, false, true, newExceptionError("MathException", "Divide by 0")
		}
		quotient := new(big.Rat).Quo(dividend, divisor)
		rounded, err := roundRatToScale("Decimal.divide", quotient, args[1].Int, mode)
		if err != nil {
			if mode == "UNNECESSARY" {
				return Null, receiver, false, true, newExceptionError("MathException", "Scale insufficient to represent division")
			}
			return Null, receiver, false, true, err
		}
		return decimalFromRat(rounded, args[1].Int), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func isDoubleUnsupportedMember(method string) bool {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case "abs", "divide", "doublevalue", "pow", "precision", "scale", "setscale", "striptrailingzeros", "toplainstring":
		return true
	default:
		return false
	}
}

func terminatingDecimalScale(value *big.Rat) (int64, bool) {
	denominator := new(big.Int).Set(value.Denom())
	two := int64(0)
	five := int64(0)
	for new(big.Int).Mod(denominator, big.NewInt(2)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(2))
		two++
	}
	for new(big.Int).Mod(denominator, big.NewInt(5)).Sign() == 0 {
		denominator.Quo(denominator, big.NewInt(5))
		five++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}
	if two > five {
		return two, true
	}
	return five, true
}

func decimalAbsValue(value Value) (Value, error) {
	if err := ensureFiniteDecimal("Decimal.abs", value.Decimal); err != nil {
		return Null, err
	}
	if rat, ok := valueDecimalRat(value); ok {
		rat.Abs(rat)
		return decimalFromRat(rat, int64(decimalScale(value))), nil
	}
	out := Decimal(math.Abs(value.Decimal))
	if isFloatBackedDecimal(value) {
		out = decimalAsDouble(out)
	}
	if text := strings.TrimSpace(value.Text); text != "" {
		text = strings.TrimPrefix(text, "-")
		text = strings.TrimPrefix(text, "+")
		out.Text = text
	}
	return out, nil
}

func int32FromDecimalValue(name string, value Value) (int32, error) {
	if rat, ok := valueDecimalRat(value); ok {
		integer := new(big.Int).Quo(rat.Num(), rat.Denom())
		if integer.Cmp(big.NewInt(-2147483648)) < 0 || integer.Cmp(big.NewInt(2147483648)) >= 0 {
			return 0, fmt.Errorf("%s value out of Integer range", name)
		}
		return int32(integer.Int64()), nil // #nosec G115 -- integer is range-checked above before narrowing.
	}
	return int32FromFloat(name, value.Decimal)
}

func int64FromDecimalValue(name string, value Value) (int64, error) {
	if rat, ok := valueDecimalRat(value); ok {
		integer := new(big.Int).Quo(rat.Num(), rat.Denom())
		if !integer.IsInt64() {
			return 0, fmt.Errorf("%s value out of 64-bit integer range", name)
		}
		return integer.Int64(), nil
	}
	return int64FromFloat(name, value.Decimal)
}

func formatDecimalTextWithGrouping(text string) string {
	sign := ""
	if strings.HasPrefix(text, "-") || strings.HasPrefix(text, "+") {
		sign = text[:1]
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
