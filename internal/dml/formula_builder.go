package dml

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// FormulaError preserves the public exception separately from unsupported local
// evaluation. Its zero value means evaluation did not raise a native error.
type FormulaError struct {
	Type, Message string
}

func (p *formulaParser) formulaFailure(kind, message string) {
	if p.failure != nil && p.failure.Type == "" {
		p.failure.Type, p.failure.Message = kind, message
	}
}

// ValidateFormulaInstance uses the evaluator's grammar with typed placeholders,
// rather than executing a sample record. Both IF branches are checked, and
// domain errors such as SQRT(-1) remain evaluation concerns (R028/R031/R044/R097).
func ValidateFormulaInstance(formula string, org *storage.OrgState, definition storage.ObjectDefinition, returnType string, template bool) string {
	if template {
		for {
			start := strings.Index(formula, "{!")
			if start < 0 {
				return ""
			}
			formula = formula[start+2:]
			end := formulaTemplateExpressionEnd(formula)
			if end < 0 {
				return "Syntax error.  Found 'end of formula'"
			}
			if message := validateFormulaInstanceExpression(formula[:end], org, definition, ""); message != "" {
				return message
			}
			formula = formula[end+1:]
		}
	}
	return validateFormulaInstanceExpression(formula, org, definition, returnType)
}

func validateFormulaInstanceExpression(formula string, org *storage.OrgState, definition storage.ObjectDefinition, returnType string) string {
	// Non-template merge syntax still exposes the field types to operator checks
	// (R211). Template literals are handled by the template path above.
	tokens := tokenizeFormula(html.UnescapeString(formula))
	var expression []formulaToken
	merge := false
	for i := 0; i < len(tokens); i++ {
		if tokens[i].typ == formulaTokenSymbol && tokens[i].text == "{" && i+1 < len(tokens) && tokens[i+1].text == "!" {
			merge = true
			i++
			continue
		}
		if merge && tokens[i].typ == formulaTokenSymbol && tokens[i].text == "}" {
			merge = false
			continue
		}
		expression = append(expression, tokens[i])
	}
	p := formulaParser{tokens: expression, org: org, definition: definition, validating: true, formulaSurface: true}
	value, ok := p.parseExpression()
	if p.validationMessage != "" {
		return p.validationMessage
	}
	if !ok || p.peek().typ != formulaTokenEOF {
		return "Syntax error.  Found 'end of formula'"
	}
	want := strings.ToUpper(returnType)
	if want == "" || value.kind == formulaNull {
		return ""
	}
	compatible := false
	switch want {
	case "DECIMAL", "DOUBLE", "INTEGER", "LONG":
		compatible = value.kind == formulaNumber
	case "BOOLEAN":
		compatible = value.kind == formulaBool
	case "STRING", "ID":
		compatible = value.kind == formulaString && value.fieldType != storage.FieldTime
	case "DATE":
		compatible = value.kind == formulaDate && value.fieldType != storage.FieldDateTime
	case "DATETIME":
		compatible = value.kind == formulaDate && value.fieldType == storage.FieldDateTime
	case "TIME":
		compatible = value.fieldType == storage.FieldTime
	}
	if compatible {
		return ""
	}
	expected := map[string]string{"DECIMAL": "Decimal", "DOUBLE": "Double", "INTEGER": "Integer", "LONG": "Long", "BOOLEAN": "Boolean", "STRING": "Text", "ID": "Id", "DATE": "Date", "DATETIME": "Date/Time", "TIME": "Time"}[want]
	return fmt.Sprintf("Formula result is data type (%s), incompatible with expected data type (%s).", formulaValueTypeName(value), expected)
}

func formulaValueTypeName(v formulaValue) string {
	switch v.kind {
	case formulaNumber:
		return "Number"
	case formulaBool:
		return "Boolean"
	case formulaDate:
		if v.fieldType == storage.FieldDateTime {
			return "Date/Time"
		}
		return "Date"
	case formulaString:
		if v.fieldType == storage.FieldTime {
			return "Time"
		}
		if v.fieldType == storage.FieldPicklist {
			return "Picklist"
		}
		return "Text"
	default:
		return "Null"
	}
}

func (p *formulaParser) formulaFieldType(name string) formulaValue {
	fieldName, ok := storage.ResolveFieldName(p.definition, "", name)
	field := p.definition.Fields[fieldName]
	if !ok && p.org != nil {
		// Apex contexts may expose flattened fields, so retain direct lookup
		// first. SObject paths use the evaluator's relationship graph instead
		// of resolving the entire dotted path as one field (R277/R278).
		parts := strings.Split(name, ".")
		if len(parts) > 1 {
			namespace := p.resolutionNamespace(p.definition)
			if lookup, found := relationshipLookupField(p.definition, namespace, parts[0]); found {
				field, _, ok = formulaRelationshipFieldDefinition(*p.org, lookup, parts[1:], namespace)
			}
		}
	}
	if !ok {
		p.validationMessage = "Could not access the following field: " + name + ". Contact your administrator."
		return formulaValue{kind: formulaNull}
	}
	v := formulaValue{fieldType: field.Type, display: strings.ToUpper(field.DisplayType)}
	typ := field.Type
	if typ == storage.FieldCalculated {
		typ = storage.FieldType(v.display)
	}
	switch typ {
	case storage.FieldDecimal, storage.FieldInteger, "CURRENCY", "PERCENT", "DOUBLE":
		v.kind = formulaNumber
	case storage.FieldBoolean:
		v.kind = formulaBool
	case storage.FieldDate, storage.FieldDateTime:
		v.kind = formulaDate
		v.fieldType = typ
	default:
		v.kind = formulaString
	}
	return v
}

// Currency fields preserve a decimal result scale on the Formula surface,
// whether read directly (R210) or through a relationship (R278).
func (p *formulaParser) formulaSurfaceFieldValue(value formulaValue) formulaValue {
	if p.formulaSurface && value.kind == formulaNumber && strings.EqualFold(value.display, "CURRENCY") && !strings.Contains(value.numberText, ".") {
		value.numberText += ".0"
	}
	return value
}

func (p *formulaParser) formulaOperatorType(op string, left, right formulaValue) (formulaValue, bool) {
	if op == "+" && left.kind == formulaString && right.kind == formulaString {
		return formulaValue{kind: formulaString}, true
	}
	if (op == "+" || op == "-") && left.kind == formulaDate && right.kind == formulaNumber {
		return left, true
	}
	if op == "-" && left.kind == formulaDate && right.kind == formulaDate {
		return formulaValue{kind: formulaNumber}, true
	}
	for _, v := range []formulaValue{left, right} {
		if v.kind != formulaNumber && v.kind != formulaNull {
			p.validationMessage = fmt.Sprintf("Incorrect parameter type for operator '%s'. Expected Number, received %s", op, formulaValueTypeName(v))
			return formulaValue{}, false
		}
	}
	return formulaValue{kind: formulaNumber}, true
}

func (p *formulaParser) formulaFunctionType(name string, args []formulaValue) (formulaValue, bool) {
	name = strings.ToUpper(name)
	counts := map[string]int{
		"NOT": 1, "ISNEW": 0, "ISBLANK": 1, "ISNULL": 1, "BLANKVALUE": 2, "NULLVALUE": 2,
		"CONTAINS": 2, "BEGINS": 2, "LEFT": 2, "RIGHT": 2, "MID": 3, "CASESAFEID": 1, "SUBSTITUTE": 3,
		"BR": 0, "REGEX": 2, "ISPICKVAL": 2, "TEXT": 1, "LOWER": 1, "UPPER": 1, "TRIM": 1,
		"LPAD": 3, "RPAD": 3, "TODAY": 0, "NOW": 0, "DATE": 3, "DAY": 1, "MONTH": 1, "YEAR": 1,
		"ADDMONTHS": 2, "DATEVALUE": 1, "DATETIMEVALUE": 1, "TIMEVALUE": 1, "WEEKDAY": 1,
		"FLOOR": 1, "CEILING": 1, "ROUND": 2, "ABS": 1, "MOD": 2, "LEN": 1, "SQRT": 1,
		"EXP": 1, "LN": 1, "LOG": 1, "POWER": 2, "SIGN": 3, "VALUE": 1, "ISNUMBER": 1, "IF": 3,
	}
	count, known := counts[name]
	if known && len(args) != count {
		p.validationMessage = fmt.Sprintf("Incorrect number of parameters for function '%s()'. Expected %d, received %d", name, count, len(args))
		return formulaValue{}, false
	}
	if !known && name != "AND" && name != "OR" && name != "CASE" && name != "MIN" && name != "MAX" && name != "IMAGE" {
		p.validationMessage = "Unknown function " + name + ". Check spelling."
		return formulaValue{}, false
	}
	if name == "IF" {
		if args[1].kind != args[2].kind && args[1].kind != formulaNull && args[2].kind != formulaNull {
			p.validationMessage = fmt.Sprintf("Incorrect parameter type for function 'IF()'. Expected %s, received %s", formulaValueTypeName(args[1]), formulaValueTypeName(args[2]))
			return formulaValue{}, false
		}
		if args[1].kind == formulaNull {
			return args[2], true
		}
		return args[1], true
	}
	if name == "TEXT" && args[0].kind == formulaBool {
		p.validationMessage = "Incorrect parameter type for function 'TEXT()'. Expected Number, Date, Date/Time, Picklist, received Boolean"
		return formulaValue{}, false
	}
	switch name {
	case "AND", "OR", "NOT", "ISNEW", "ISBLANK", "ISNULL", "CONTAINS", "BEGINS", "REGEX", "ISPICKVAL", "ISNUMBER":
		return formulaValue{kind: formulaBool}, true
	case "BLANKVALUE", "NULLVALUE":
		return args[0], true
	case "CASE":
		if len(args) < 4 {
			return formulaValue{}, false
		}
		return args[2], true
	case "DATE", "TODAY", "DATEVALUE", "ADDMONTHS":
		return formulaValue{kind: formulaDate, fieldType: storage.FieldDate}, true
	case "NOW", "DATETIMEVALUE":
		return formulaValue{kind: formulaDate, fieldType: storage.FieldDateTime}, true
	case "TIMEVALUE":
		return formulaValue{kind: formulaString, fieldType: storage.FieldTime}, true
	case "DAY", "MONTH", "YEAR", "WEEKDAY", "FLOOR", "CEILING", "ROUND", "ABS", "MOD", "LEN", "SQRT", "EXP", "LN", "LOG", "POWER", "SIGN", "VALUE", "MIN", "MAX":
		return formulaValue{kind: formulaNumber}, true
	default:
		return formulaValue{kind: formulaString}, true
	}
}

func (p *formulaParser) preciseNumberBinary(op string, left, right formulaValue) (formulaValue, bool) {
	if !p.formulaSurface {
		return preciseFormulaNumberBinary(op, left, right)
	}
	a, aOK := formulaNumberRat(left)
	b, bOK := formulaNumberRat(right)
	if !aOK || !bOK {
		return formulaValue{}, false
	}
	scale := max(formulaNumberScale(left), formulaNumberScale(right))
	switch op {
	case "+":
		a.Add(a, b)
	case "-":
		a.Sub(a, b)
	case "*":
		a.Mul(a, b)
		scale = formulaNumberScale(left) + formulaNumberScale(right)
	default:
		return formulaValue{}, false
	}
	text := a.FloatString(scale)
	number, err := strconv.ParseFloat(text, 64)
	return formulaValue{kind: formulaNumber, number: number, numberText: text}, err == nil
}

func materializedFormulaSurfaceValue(field storage.Field, value formulaValue) (storage.Value, bool, bool) {
	if value.kind == formulaNull {
		return storage.NullValue(), true, true
	}
	if value.kind == formulaString && value.text == "" && value.fieldType == "" {
		return storage.NullValue(), true, true
	}
	typ := field.Type
	if typ == storage.FieldCalculated {
		typ = storage.FieldType(strings.ToUpper(field.DisplayType))
	}
	switch typ {
	case storage.FieldString, storage.FieldPicklist, storage.FieldTime:
		return storage.StringValue(value.asString()), false, true
	case storage.FieldDecimal, "DOUBLE", "CURRENCY", "PERCENT":
		if field.Type == storage.FieldCalculated && strings.EqualFold(field.DisplayType, "CURRENCY") {
			return materializedFormulaFieldValue(field, value)
		}
		text := value.asString()
		if field.Type == storage.FieldCalculated && value.kind == formulaNumber && field.Scale > 0 && formulaNumberScale(value) > field.Scale {
			if number, ok := formulaNumberRat(value); ok {
				text = number.FloatString(field.Scale)
			}
		}
		return storage.DecimalValue(text), false, true
	case storage.FieldInteger:
		return storage.IntegerValue(int64(value.number)), false, true
	case storage.FieldBoolean:
		return storage.BooleanValue(value.bool), false, true
	case storage.FieldDate:
		return storage.DateValue(value.asString()), false, true
	case storage.FieldDateTime:
		return storage.DateTimeValue(value.asString()), false, true
	case storage.FieldID:
		return storage.IDValue(storage.ID(value.asString())), false, true
	default:
		return materializedFormulaFieldValue(field, value)
	}
}

func (p *formulaParser) evaluateExtendedFormulaFunction(name string, args []formulaValue) (formulaValue, bool, bool) {
	name = strings.ToUpper(name)
	numberResult := func(n float64, decimal bool) formulaValue {
		text := strconv.FormatFloat(n, 'f', -1, 64)
		if decimal && !strings.Contains(text, ".") {
			text += ".0"
		}
		return formulaValue{kind: formulaNumber, number: n, numberText: text}
	}
	switch name {
	case "CEILING", "SQRT", "EXP", "LN", "LOG":
		if len(args) != 1 {
			return formulaValue{}, true, false
		}
		if p.preserveNumericNull && args[0].kind == formulaNull {
			return args[0], true, true
		}
		n, ok := args[0].asNumber()
		if !ok {
			return formulaValue{}, true, false
		}
		switch name {
		case "CEILING":
			n = math.Copysign(math.Ceil(math.Abs(n)), n)
		case "SQRT":
			n = math.Sqrt(n)
		case "EXP":
			n = math.Exp(n)
		case "LN":
			n = math.Log(n)
		case "LOG":
			n = math.Log10(n)
		}
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return formulaValue{}, true, false
		}
		return numberResult(n, p.formulaSurface && name != "CEILING"), true, true
	case "POWER", "MIN", "MAX":
		if len(args) == 0 || (name == "POWER" && len(args) != 2) {
			return formulaValue{}, true, false
		}
		n, ok := args[0].asNumber()
		if !ok {
			return formulaValue{}, true, false
		}
		for _, a := range args[1:] {
			v, ok := a.asNumber()
			if !ok {
				return formulaValue{}, true, false
			}
			switch name {
			case "POWER":
				n = math.Pow(n, v)
			case "MIN":
				n = math.Min(n, v)
			case "MAX":
				n = math.Max(n, v)
			}
		}
		return numberResult(n, p.formulaSurface && name == "POWER"), true, true
	case "VALUE", "ISNUMBER":
		if len(args) != 1 {
			return formulaValue{}, true, false
		}
		text := strings.TrimSpace(args[0].asString())
		n, err := strconv.ParseFloat(text, 64)
		if name == "ISNUMBER" {
			return formulaValue{kind: formulaBool, bool: err == nil}, true, true
		}
		if text == "" {
			return formulaValue{kind: formulaNull}, true, true
		}
		return formulaValue{kind: formulaNumber, number: n, numberText: text}, true, err == nil
	case "TRIM":
		if len(args) != 1 {
			return formulaValue{}, true, false
		}
		return formulaValue{kind: formulaString, text: strings.TrimSpace(args[0].asString())}, true, true
	case "LPAD", "RPAD":
		if len(args) != 3 {
			return formulaValue{}, true, false
		}
		n, ok := formulaIntArg(args[1])
		if !ok || n < 0 {
			return formulaValue{}, true, false
		}
		text, pad := args[0].asString(), args[2].asString()
		if n <= len(text) {
			return formulaValue{kind: formulaString, text: text[:n]}, true, true
		}
		if pad == "" {
			return formulaValue{kind: formulaString, text: text}, true, true
		}
		missing := n - len(text)
		padding := strings.Repeat(pad, (missing+len(pad)-1)/len(pad))[:missing]
		if name == "LPAD" {
			text = padding + text
		} else {
			text += padding
		}
		return formulaValue{kind: formulaString, text: text}, true, true
	case "ADDMONTHS", "WEEKDAY", "DATEVALUE", "DATETIMEVALUE":
		count := 1
		if name == "ADDMONTHS" {
			count = 2
		}
		if len(args) != count {
			return formulaValue{}, true, false
		}
		if args[0].kind == formulaNull {
			return formulaValue{kind: formulaNull}, true, true
		}
		text := args[0].asString()
		date, ok := parseFormulaDateTime(text)
		if !ok {
			date, ok = parseFormulaDate(text)
		}
		if !ok {
			p.formulaFailure("HandledException", "Invalid year for DATEVALUE function")
			return formulaValue{}, true, false
		}
		switch name {
		case "ADDMONTHS":
			n, ok := formulaIntArg(args[1])
			if !ok {
				return formulaValue{}, true, false
			}
			end := date.Day() == time.Date(date.Year(), date.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			first := time.Date(date.Year(), date.Month()+time.Month(n), 1, 0, 0, 0, 0, time.UTC)
			last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			day := min(date.Day(), last)
			if end {
				day = last
			}
			date = time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
		case "WEEKDAY":
			return numberResult(float64(date.Weekday())+1, false), true, true
		case "DATETIMEVALUE":
			return formulaValue{kind: formulaDate, fieldType: storage.FieldDateTime, text: date.UTC().Format(time.RFC3339Nano)}, true, true
		}
		return formulaValue{kind: formulaDate, fieldType: storage.FieldDate, text: date.Format("2006-01-02")}, true, true
	case "TIMEVALUE":
		if len(args) != 1 {
			return formulaValue{}, true, false
		}
		if args[0].kind == formulaNull {
			return formulaValue{kind: formulaNull}, true, true
		}
		text := args[0].asString()
		value, err := time.Parse("15:04:05.999", strings.TrimSuffix(text, "Z"))
		if err != nil {
			p.formulaFailure("HandledException", "Invalid time format: "+text)
			return formulaValue{}, true, false
		}
		return formulaValue{kind: formulaString, fieldType: storage.FieldTime, text: value.Format("15:04:05.000") + "Z"}, true, true
	}
	return formulaValue{}, false, false
}
