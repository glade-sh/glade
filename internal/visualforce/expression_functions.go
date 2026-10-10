package visualforce

import (
	"fmt"
	"html"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/vm"
)

// formulaEvaluationError identifies failures that native Visualforce reports
// with the containing expression/component/page. Controller exceptions retain
// their own message and are not converted into formula errors.
type formulaEvaluationError struct {
	cause string
}

func (err *formulaEvaluationError) Error() string {
	if err.cause != "" {
		return err.cause
	}
	return "Error evaluating Visualforce expression"
}

func failVisualforceFormula(ctx *ExpressionContext, cause string) *vm.Value {
	if ctx != nil && ctx.evaluationError == nil {
		ctx.evaluationError = &formulaEvaluationError{cause: cause}
	}
	return &vm.Null
}

func visualforceFormulaText(value vm.Value) string {
	if value.Kind == vm.ValueNull {
		return ""
	}
	return value.String()
}

// Use the operands' decimal text for arithmetic, so 0.1 + 0.2 does not expose
// the binary floating-point tail (native r_op_decimal). Retain integer results
// and use the existing numeric fallback for non-decimal operands.
func exactVisualforceArithmetic(op string, left, right vm.Value) (vm.Value, bool) {
	a, ok := new(big.Rat).SetString(left.String())
	if !ok {
		return vm.Null, false
	}
	b, ok := new(big.Rat).SetString(right.String())
	if !ok {
		return vm.Null, false
	}
	value := new(big.Rat)
	switch op {
	case "+":
		value.Add(a, b)
	case "-":
		value.Sub(a, b)
	case "*":
		value.Mul(a, b)
	case "/":
		if b.Sign() == 0 {
			return vm.Null, false
		}
		value.Quo(a, b)
	default:
		return vm.Null, false
	}
	if left.Kind == vm.ValueInt && right.Kind == vm.ValueInt && value.IsInt() && value.Num().IsInt64() {
		return vm.Int(value.Num().Int64()), true
	}
	// A finite decimal denominator consists only of powers of two and five.
	// Compute its exact scale; leave recurring divisions on the prior path.
	denominator := new(big.Int).Set(value.Denom())
	scale := 0
	for _, factor := range []int64{2, 5} {
		count := 0
		divisor := big.NewInt(factor)
		for {
			quotient, remainder := new(big.Int), new(big.Int)
			quotient.QuoRem(denominator, divisor, remainder)
			if remainder.Sign() != 0 {
				break
			}
			denominator = quotient
			count++
		}
		if count > scale {
			scale = count
		}
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return vm.Null, false
	}
	number, _ := value.Float64()
	out := vm.Decimal(number)
	out.Text = value.FloatString(scale)
	return out, true
}

// These numeric and text functions are backed by owned native function cases.
func evalVisualforceFormulaFunction(ctx *ExpressionContext, name string, args []Expression) (*vm.Value, bool) {
	name = strings.ToUpper(name)
	switch name {
	case "ABS", "CEILING", "FLOOR", "ROUND", "MAX", "MIN", "MOD", "SQRT", "EXP", "LN",
		"LEN", "TRIM", "LEFT", "RIGHT", "MID", "FIND", "CONTAINS", "BEGINS", "SUBSTITUTE", "LPAD", "RPAD",
		"NOW", "TODAY", "DATETIMEVALUE", "DATEVALUE", "DAY", "MONTH", "YEAR":
	default:
		return nil, false
	}
	values := make([]vm.Value, 0, len(args))
	for _, arg := range args {
		value := evalExpressionValue(arg, ctx)
		if expressionEvaluationFailed(ctx) {
			return &vm.Null, true
		}
		values = append(values, value)
	}
	value := func(i int) vm.Value {
		if i >= len(values) {
			return vm.Null
		}
		return values[i]
	}
	text := func(i int) string { return visualforceFormulaText(value(i)) }
	number := func(i int) float64 { n, _ := numericValue(value(i)); return n }
	// Captures use numeric formula arguments, not text-to-number coercions.
	// Keep the prior fallback for other argument kinds until they are captured.
	numericArgs := []int(nil)
	switch name {
	case "ABS", "CEILING", "FLOOR", "SQRT", "EXP", "LN":
		numericArgs = []int{0}
	case "ROUND", "MOD":
		numericArgs = []int{0, 1}
	case "MAX", "MIN":
		for i := range values {
			numericArgs = append(numericArgs, i)
		}
	case "LEFT", "RIGHT", "LPAD", "RPAD":
		numericArgs = []int{1}
	case "MID":
		numericArgs = []int{1, 2}
	}
	for _, i := range numericArgs {
		if v := value(i); v.Kind != vm.ValueInt && v.Kind != vm.ValueDecimal {
			out := value(0)
			return &out, true
		}
	}
	// FIND's captured form has two arguments; preserve the old fallback for
	// the unobserved optional-position form rather than silently ignoring it.
	if name == "FIND" && len(values) != 2 {
		out := value(0)
		return &out, true
	}
	out := vm.Null
	switch name {
	case "ABS":
		out = numericResult(value(0), vm.Int(0), math.Abs(number(0)))
	case "CEILING":
		out = vm.Int(int64(math.Copysign(math.Ceil(math.Abs(number(0))), number(0))))
	case "FLOOR":
		out = vm.Int(int64(math.Trunc(number(0))))
	case "ROUND":
		factor := math.Pow10(int(number(1)))
		out = vm.Decimal(math.Round(number(0)*factor) / factor)
	case "MAX", "MIN":
		if len(values) != 0 {
			out = values[0]
			best := number(0)
			for i := 1; i < len(values); i++ {
				n := number(i)
				if (name == "MAX" && n > best) || (name == "MIN" && n < best) {
					best, out = n, values[i]
				}
			}
		}
	case "MOD":
		if number(1) == 0 {
			return failVisualforceFormula(ctx, "Arithmetic error: Division by zero"), true
		}
		out = numericResult(value(0), value(1), math.Mod(number(0), number(1)))
	case "SQRT", "EXP", "LN":
		n := number(0)
		switch name {
		case "SQRT":
			if n < 0 {
				return failVisualforceFormula(ctx, ""), true
			}
			n = math.Sqrt(n)
		case "EXP":
			n = math.Exp(n)
		case "LN":
			n = math.Log(n)
		}
		out = vm.Decimal(n)
		if math.Trunc(n) == n && !math.IsInf(n, 0) {
			out.Text = strconv.FormatFloat(n, 'f', 1, 64)
		}
	case "LEN":
		out = vm.Int(int64(utf8.RuneCountInString(text(0))))
	case "TRIM":
		out = vm.String(strings.TrimSpace(text(0)))
	case "LEFT", "RIGHT", "MID":
		runes := []rune(text(0))
		start, count := 0, int(number(1))
		if name == "RIGHT" {
			start = len(runes) - count
		} else if name == "MID" {
			start, count = int(number(1))-1, int(number(2))
		}
		if start < 0 {
			start = 0
		}
		if start > len(runes) {
			start = len(runes)
		}
		if count < 0 {
			count = 0
		}
		if count > len(runes)-start {
			count = len(runes) - start
		}
		out = vm.String(string(runes[start : start+count]))
	case "FIND":
		index := strings.Index(text(1), text(0))
		if index < 0 {
			out = vm.Int(0)
		} else {
			out = vm.Int(int64(utf8.RuneCountInString(text(1)[:index]) + 1))
		}
	case "CONTAINS":
		out = vm.Bool(strings.Contains(text(0), text(1)))
	case "BEGINS":
		out = vm.Bool(strings.HasPrefix(text(0), text(1)))
	case "SUBSTITUTE":
		if text(1) != "" {
			out = vm.String(strings.ReplaceAll(text(0), text(1), text(2)))
		} else {
			out = value(0)
		}
	case "LPAD", "RPAD":
		out = vm.String(text(0))
		missing := int(number(1)) - utf8.RuneCountInString(text(0))
		pad := text(2)
		if len(values) < 3 {
			pad = " "
		}
		if missing > 0 && pad != "" {
			var padding strings.Builder
			runes := []rune(pad)
			for i := 0; i < missing; i++ {
				padding.WriteRune(runes[i%len(runes)])
			}
			p := padding.String()
			if name == "LPAD" {
				out = vm.String(p + text(0))
			} else {
				out = vm.String(text(0) + p)
			}
		}
	case "NOW", "TODAY":
		out = vm.Object("Datetime")
		out.Text = time.Now().UTC().Format("2006-01-02 15:04:05")
		if name == "TODAY" {
			out.Type = "Date"
			out.Text = out.Text[:10]
		}
		out.Fields["value"] = vm.String(out.Text)
	case "DATETIMEVALUE":
		if date, err := time.Parse("2006-01-02 15:04:05", text(0)); err == nil {
			out = vm.Object("Datetime")
			out.Text = date.Format("2006-01-02 15:04:05")
			out.Fields["value"] = vm.String(out.Text)
		}
	case "DATEVALUE":
		if strings.EqualFold(value(0).Type, "Datetime") && len(text(0)) >= 10 {
			out = vm.Object("Date")
			out.Text = text(0)[:10]
			out.Fields["value"] = vm.String(out.Text)
		}
	case "DAY", "MONTH", "YEAR":
		if strings.EqualFold(value(0).Type, "Date") {
			if date, err := time.Parse("2006-01-02", text(0)); err == nil {
				n := date.Day()
				if name == "MONTH" {
					n = int(date.Month())
				} else if name == "YEAR" {
					n = date.Year()
				}
				out = vm.Int(int64(n))
			}
		}
	}
	return &out, true
}

func escapeVisualforceFormulaJavaScript(raw string) string {
	// JSENCODE differs from the script transport helper: angle brackets are
	// Unicode escapes and an ordinary slash is unchanged (r_escape_jsencode_*).
	return strings.NewReplacer("<", `\u003C`, ">", `\u003E`).Replace(
		strings.ReplaceAll(EscapeVisualforceJavaScriptString(raw), `<\/`, `</`))
}

func escapeVisualforceFormulaJSInHTML(raw string) string {
	value := strings.NewReplacer(`\`, `\\`, "'", `\'`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(raw)
	return html.EscapeString(value)
}

func contextualVisualforceFormulaError(err error, formula *formulaEvaluationError, raw, component, page string) error {
	message := fmt.Sprintf("Error is in expression '%s' in component <%s> in page %s", raw, component, page)
	if formula.cause != "" {
		message = formula.cause + "\n" + message
	}
	return &contextualFormulaError{message: message, cause: err}
}

type contextualFormulaError struct {
	message string
	cause   error
}

func (err *contextualFormulaError) Error() string { return err.message }
func (err *contextualFormulaError) Unwrap() error { return err.cause }
