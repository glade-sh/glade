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

func decimalAbsValue(value Value) (Value, error) {
	if err := ensureFiniteDecimal("Decimal.abs", value.Decimal); err != nil {
		return Null, err
	}
	out := Decimal(math.Abs(value.Decimal))
	if text := strings.TrimSpace(value.Text); text != "" {
		text = strings.TrimPrefix(text, "-")
		text = strings.TrimPrefix(text, "+")
		out.Text = text
	}
	return out, nil
}
