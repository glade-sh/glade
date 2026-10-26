package visualforce

import (
	"fmt"
	"html"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/vm"
)

// Keep parameter values typed until formatting. Flattening them through a text
// template loses the distinction between numbers, dates, strings and nulls.
func renderOutputTextParameters(node *MarkupNode, ctx *RenderContext, pattern string, literalPattern, escape bool) (string, bool, error) {
	var args []vm.Value
	for _, child := range node.Children {
		if !isApexStructureTag(child, "param") {
			continue
		}
		raw := child.Attribute("value")
		value := vm.Null
		var err error
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "{!") && findExpressionTemplateEnd(trimmed, 2) == len(trimmed)-1 {
			value, _, err = evaluateRenderExpressionValue(raw, ctx)
		} else {
			var text string
			text, err = RenderExpressionTemplate(raw, ctx.Expression)
			value = vm.String(text)
		}
		if err != nil {
			return "", true, err
		}
		args = append(args, value)
	}
	if len(args) == 0 {
		return "", false, nil
	}
	var out strings.Builder
	writeLiteral := func(text string) {
		// Literal markup in the value attribute stays text even with escape=false;
		// parameter markup follows escape. Never parse substituted values again.
		if escape || literalPattern {
			text = html.EscapeString(text)
		}
		out.WriteString(text)
	}
	quoted := false
	for i := 0; i < len(pattern); {
		if pattern[i] == '\'' {
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				writeLiteral("'")
				i += 2
			} else {
				quoted = !quoted
				i++
			}
			continue
		}
		if pattern[i] != '{' || quoted {
			writeLiteral(pattern[i : i+1])
			i++
			continue
		}
		end := strings.IndexByte(pattern[i+1:], '}')
		if end < 0 {
			return "", true, fmt.Errorf("unmatched '{' in outputText format")
		}
		end += i + 1
		parts := strings.SplitN(pattern[i+1:end], ",", 3)
		index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || index < 0 {
			return "", true, fmt.Errorf("invalid outputText argument index %q", parts[0])
		}
		if index >= len(args) {
			writeLiteral("{" + strconv.Itoa(index) + "}")
		} else {
			text, err := formatOutputTextArgument(args[index], parts[1:], ctx)
			if err != nil {
				return "", true, err
			}
			if escape {
				text = html.EscapeString(text)
			}
			out.WriteString(text)
		}
		i = end + 1
	}
	return out.String(), true, nil
}

func formatOutputTextArgument(value vm.Value, format []string, ctx *RenderContext) (string, error) {
	if value.Kind == vm.ValueNull {
		return "", nil
	}
	kind, style := "", ""
	if len(format) > 0 {
		kind = strings.TrimSpace(format[0])
	}
	if len(format) > 1 {
		style = format[1]
	}
	if kind == "" {
		if value.Kind != vm.ValueInt && value.Kind != vm.ValueDecimal {
			return value.String(), nil
		}
		kind = "number"
	}
	switch kind {
	case "number":
		if value.Kind != vm.ValueInt && value.Kind != vm.ValueDecimal {
			return "", fmt.Errorf("Cannot format given Object as a Number")
		}
		if controllerFieldLocale(ctx) != "en_US" {
			return "", vm.NewUnsupportedFeatureError("outputText number format locale " + controllerFieldLocale(ctx))
		}
		if style == "" {
			style = "#,##0.###"
		}
		return formatOutputTextNumber(value, style)
	case "date", "time":
		locale := controllerFieldLocale(ctx)
		if locale == "en_US" {
			style = outputTextDateStyle(kind, style)
		}
		return formatOutputTextDateForLocale(value, style, locale)
	default:
		return "", vm.NewUnsupportedFeatureError("outputText format type " + kind)
	}
}

// The numeric profile uses DecimalFormat's digit placeholders, grouping and
// literal affixes. Native rows establish half-even rounding, including ties.
func formatOutputTextNumber(value vm.Value, pattern string) (string, error) {
	switch strings.TrimSpace(pattern) {
	case "integer":
		pattern = "#,##0"
	case "currency":
		pattern = "¤#,##0.00"
	case "percent":
		pattern = "#,##0%"
	}
	positive, negativePattern := pattern, ""
	quoted := false
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\'' {
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				i++
			} else {
				quoted = !quoted
			}
		} else if pattern[i] == ';' && !quoted {
			positive, negativePattern = pattern[:i], pattern[i+1:]
			break
		}
	}
	prefix, numeric, suffix, multiplier, err := outputTextNumberParts(positive)
	if err != nil {
		return "", err
	}
	exponentWidth := 0
	if parts := strings.Split(numeric, "E"); len(parts) > 1 {
		if len(parts) != 2 || parts[1] == "" || strings.Trim(parts[1], "0") != "" || len(parts[1]) > 6 || len(strings.Split(parts[0], ".")[0]) != 1 {
			return "", vm.NewUnsupportedFeatureError("outputText scientific number pattern")
		}
		numeric, exponentWidth = parts[0], len(parts[1])
	}
	digits := strings.Split(numeric, ".")
	if len(digits) > 2 {
		return "", fmt.Errorf("invalid outputText number pattern")
	}
	minWhole := strings.Count(digits[0], "0")
	minFraction, maxFraction := 0, 0
	if len(digits) == 2 {
		minFraction = strings.Count(digits[1], "0")
		maxFraction = len(digits[1])
	}
	if maxFraction > 33 || strings.ContainsAny(strings.Join(digits[1:], ""), ",.") {
		return "", vm.NewUnsupportedFeatureError("outputText number fraction pattern")
	}
	number, ok := new(big.Rat).SetString(value.String())
	if !ok {
		return "", fmt.Errorf("Cannot format given Object as a Number")
	}
	negative := number.Sign() < 0
	if negative && negativePattern != "" {
		prefix, _, suffix, _, err = outputTextNumberParts(negativePattern)
		if err != nil {
			return "", err
		}
	} else if negative {
		prefix = "-" + prefix
	}
	number.Abs(number)
	number.Mul(number, big.NewRat(int64(multiplier), 1))
	exponent := 0
	if exponentWidth > 0 && number.Sign() != 0 {
		for number.Cmp(big.NewRat(10, 1)) >= 0 {
			number.Quo(number, big.NewRat(10, 1))
			exponent++
		}
		for number.Cmp(big.NewRat(1, 1)) < 0 {
			number.Mul(number, big.NewRat(10, 1))
			exponent--
		}
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(maxFraction)), nil)
	number.Mul(number, new(big.Rat).SetInt(factor))
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(number.Num(), number.Denom(), remainder)
	cmp := new(big.Int).Lsh(remainder, 1).Cmp(number.Denom())
	if cmp > 0 || (cmp == 0 && whole.Bit(0) == 1) {
		whole.Add(whole, big.NewInt(1))
	}
	if exponentWidth > 0 && whole.Cmp(new(big.Int).Mul(factor, big.NewInt(10))) >= 0 {
		whole.Quo(whole, big.NewInt(10))
		exponent++
	}
	text := whole.String()
	if len(text) <= maxFraction {
		text = strings.Repeat("0", maxFraction+1-len(text)) + text
	}
	fraction := ""
	if maxFraction > 0 {
		fraction = text[len(text)-maxFraction:]
		text = text[:len(text)-maxFraction]
		for len(fraction) > minFraction && fraction[len(fraction)-1] == '0' {
			fraction = fraction[:len(fraction)-1]
		}
	}
	if len(text) < minWhole {
		text = strings.Repeat("0", minWhole-len(text)) + text
	}
	if comma := strings.LastIndexByte(digits[0], ','); comma >= 0 {
		group := len(digits[0]) - comma - 1
		if group == 0 {
			return "", fmt.Errorf("invalid outputText grouping pattern")
		}
		for pos := len(text) - group; pos > 0; pos -= group {
			text = text[:pos] + "," + text[pos:]
		}
	}
	if fraction != "" {
		text += "." + fraction
	}
	if exponentWidth > 0 {
		sign := ""
		if exponent < 0 {
			sign, exponent = "-", -exponent
		}
		text += fmt.Sprintf("E%s%0*d", sign, exponentWidth, exponent)
	}
	return prefix + text + suffix, nil
}

// Date parameters use the GMT clock rather than the execution user's zone.
// English month/day names and named styles use the captured en_US profile.
func formatOutputTextDate(value vm.Value, pattern string) (string, error) {
	return formatOutputTextDateForLocale(value, pattern, "en_US")
}

func formatOutputTextDateForLocale(value vm.Value, pattern, locale string) (string, error) {
	if pattern == "" {
		return "", vm.NewUnsupportedFeatureError("outputText default date pattern")
	}
	if value.Kind != vm.ValueObject || (value.Type != "Date" && value.Type != "Datetime") {
		return "", fmt.Errorf("Cannot format given Object as a Date")
	}
	raw, ok := value.Fields["value"]
	if !ok {
		return "", fmt.Errorf("Cannot format given Object as a Date")
	}
	var stamp time.Time
	var err error
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02", "2006-01-02 15:04:05"} {
		stamp, err = time.Parse(layout, raw.String())
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", err
	}
	stamp = stamp.UTC()
	var out strings.Builder
	quoted := false
	for i := 0; i < len(pattern); {
		c := pattern[i]
		if c == '\'' {
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				out.WriteByte(c)
				i += 2
			} else {
				quoted = !quoted
				i++
			}
			continue
		}
		if quoted || c < 'A' || (c > 'Z' && c < 'a') || c > 'z' {
			out.WriteByte(c)
			i++
			continue
		}
		end := i + 1
		for end < len(pattern) && pattern[end] == c {
			end++
		}
		width := end - i
		if locale != "en_US" && (c == 'M' && width >= 3 || c == 'E' || c == 'a') {
			return "", vm.NewUnsupportedFeatureError("outputText localized date pattern")
		}
		n := 0
		switch c {
		case 'y':
			n = stamp.Year()
			if width == 2 {
				n %= 100
			}
		case 'M':
			if width > 2 {
				name := stamp.Month().String()
				if width == 3 {
					name = name[:3]
				}
				out.WriteString(name)
				i = end
				continue
			}
			n = int(stamp.Month())
		case 'E':
			name := stamp.Weekday().String()
			if width < 4 {
				name = name[:3]
			}
			out.WriteString(name)
			i = end
			continue
		case 'a', 'z', 'Z':
			text := "AM"
			if stamp.Hour() >= 12 {
				text = "PM"
			}
			if c == 'z' {
				text = "GMT"
			} else if c == 'Z' {
				text = "+0000"
			}
			out.WriteString(text)
			i = end
			continue
		case 'd':
			n = stamp.Day()
		case 'H':
			n = stamp.Hour()
		case 'h':
			n = stamp.Hour() % 12
			if n == 0 {
				n = 12
			}
		case 'm':
			n = stamp.Minute()
		case 's':
			n = stamp.Second()
		default:
			return "", vm.NewUnsupportedFeatureError("outputText date pattern")
		}
		out.WriteString(fmt.Sprintf("%0*d", width, n))
		i = end
	}
	return out.String(), nil
}

// Numeric affixes retain quoted literals and apply unquoted scaling/currency.
// The negative subpattern supplies affixes; the positive subpattern owns digits.
func outputTextNumberParts(pattern string) (string, string, string, int, error) {
	start := -1
	quoted := false
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\'' {
			if i+1 < len(pattern) && pattern[i+1] == '\'' {
				i++
			} else {
				quoted = !quoted
			}
		} else if !quoted && (pattern[i] == '#' || pattern[i] == '0') {
			start = i
			break
		}
	}
	if start < 0 {
		return "", "", "", 0, vm.NewUnsupportedFeatureError("outputText number pattern")
	}
	end := start
	for end < len(pattern) && strings.ContainsRune("#0,.E", rune(pattern[end])) {
		end++
	}
	prefix, prefixScale, err := outputTextNumberAffix(pattern[:start])
	if err != nil {
		return "", "", "", 0, err
	}
	suffix, suffixScale, err := outputTextNumberAffix(pattern[end:])
	if err != nil {
		return "", "", "", 0, err
	}
	if prefixScale != 1 && suffixScale != 1 {
		return "", "", "", 0, vm.NewUnsupportedFeatureError("outputText number multiplier pattern")
	}
	return prefix, pattern[start:end], suffix, prefixScale * suffixScale, nil
}

func outputTextNumberAffix(pattern string) (string, int, error) {
	var out strings.Builder
	runes := []rune(pattern)
	quoted, multiplier := false, 1
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == '\'' {
			if i+1 < len(runes) && runes[i+1] == '\'' {
				out.WriteRune(c)
				i++
			} else {
				quoted = !quoted
			}
			continue
		}
		if !quoted {
			switch c {
			case '#', '0', ';', 'E':
				return "", 0, vm.NewUnsupportedFeatureError("outputText number pattern")
			case '%', '‰':
				if multiplier != 1 {
					return "", 0, vm.NewUnsupportedFeatureError("outputText number multiplier pattern")
				}
				multiplier = 100
				if c == '‰' {
					multiplier = 1000
				}
			case '¤':
				if i+1 < len(runes) && runes[i+1] == '¤' {
					out.WriteString("USD")
					i++
				} else {
					out.WriteByte('$')
				}
				continue
			}
		}
		out.WriteRune(c)
	}
	if quoted {
		return "", 0, fmt.Errorf("unmatched quote in outputText number pattern")
	}
	return out.String(), multiplier, nil
}

func outputTextDateStyle(kind, style string) string {
	if style == "" {
		style = "medium"
	}
	if kind == "time" {
		switch style {
		case "short":
			return "h:mm a"
		case "medium":
			return "h:mm:ss a"
		case "long", "full":
			return "h:mm:ss a z"
		}
	} else {
		switch style {
		case "short":
			return "M/d/yy"
		case "medium":
			return "MMM d, yyyy"
		case "long":
			return "MMMM d, yyyy"
		case "full":
			return "EEEE, MMMM d, yyyy"
		}
	}
	return style
}
