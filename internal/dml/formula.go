package dml

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/open-aer/oaer/internal/storage"
)

type formulaKind int

const (
	formulaNull formulaKind = iota
	formulaBool
	formulaString
	formulaNumber
)

type formulaValue struct {
	kind   formulaKind
	bool   bool
	text   string
	number float64
}

func evaluateRecordFormula(formula string, record storage.Record) (bool, bool) {
	parser := formulaParser{tokens: tokenizeFormula(html.UnescapeString(formula)), record: record}
	value, ok := parser.parseExpression()
	if !ok || parser.peek().typ != formulaTokenEOF {
		return false, false
	}
	return value.truthy(), true
}

func evaluateRecordFormulaValue(formula string, field storage.Field, record storage.Record) (storage.Value, bool, bool) {
	parser := formulaParser{tokens: tokenizeFormula(html.UnescapeString(formula)), record: record}
	value, ok := parser.parseExpression()
	if !ok || parser.peek().typ != formulaTokenEOF {
		return storage.Value{}, false, false
	}
	if value.kind == formulaNull {
		return storage.NullValue(), true, true
	}
	return workflowLiteralValue(field, value.asString())
}

type formulaTokenType int

const (
	formulaTokenEOF formulaTokenType = iota
	formulaTokenIdent
	formulaTokenString
	formulaTokenNumber
	formulaTokenSymbol
)

type formulaToken struct {
	typ  formulaTokenType
	text string
}

func tokenizeFormula(input string) []formulaToken {
	tokens := []formulaToken(nil)
	for i := 0; i < len(input); {
		ch := input[i]
		if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
			i++
			continue
		}
		if ch == '\'' || ch == '"' {
			quote := ch
			start := i + 1
			i++
			var b strings.Builder
			for i < len(input) {
				if input[i] == '\\' && i+1 < len(input) && (input[i+1] == quote || input[i+1] == '\\') {
					b.WriteByte(input[i+1])
					i += 2
					continue
				}
				if input[i] == quote {
					break
				}
				b.WriteByte(input[i])
				i++
			}
			if i < len(input) && input[i] == quote {
				i++
			} else {
				b.WriteString(input[start:i])
			}
			tokens = append(tokens, formulaToken{typ: formulaTokenString, text: b.String()})
			continue
		}
		if isFormulaIdentStart(ch) {
			start := i
			i++
			for i < len(input) && isFormulaIdentPart(input[i]) {
				i++
			}
			tokens = append(tokens, formulaToken{typ: formulaTokenIdent, text: input[start:i]})
			continue
		}
		if ch >= '0' && ch <= '9' {
			start := i
			i++
			for i < len(input) && ((input[i] >= '0' && input[i] <= '9') || input[i] == '.') {
				i++
			}
			tokens = append(tokens, formulaToken{typ: formulaTokenNumber, text: input[start:i]})
			continue
		}
		if i+1 < len(input) {
			pair := input[i : i+2]
			if pair == "&&" || pair == "||" || pair == "!=" || pair == "<>" || pair == "<=" || pair == ">=" {
				tokens = append(tokens, formulaToken{typ: formulaTokenSymbol, text: pair})
				i += 2
				continue
			}
		}
		tokens = append(tokens, formulaToken{typ: formulaTokenSymbol, text: input[i : i+1]})
		i++
	}
	return append(tokens, formulaToken{typ: formulaTokenEOF})
}

func isFormulaIdentStart(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '_' || ch == '$'
}

func isFormulaIdentPart(ch byte) bool {
	return isFormulaIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '.'
}

type formulaParser struct {
	tokens []formulaToken
	pos    int
	record storage.Record
}

func (p *formulaParser) parseExpression() (formulaValue, bool) {
	return p.parseOr()
}

func (p *formulaParser) parseOr() (formulaValue, bool) {
	left, ok := p.parseAnd()
	if !ok {
		return formulaValue{}, false
	}
	for p.matchSymbol("||") {
		right, ok := p.parseAnd()
		if !ok {
			return formulaValue{}, false
		}
		left = formulaValue{kind: formulaBool, bool: left.truthy() || right.truthy()}
	}
	return left, true
}

func (p *formulaParser) parseAnd() (formulaValue, bool) {
	left, ok := p.parseComparison()
	if !ok {
		return formulaValue{}, false
	}
	for p.matchSymbol("&&") {
		right, ok := p.parseComparison()
		if !ok {
			return formulaValue{}, false
		}
		left = formulaValue{kind: formulaBool, bool: left.truthy() && right.truthy()}
	}
	return left, true
}

func (p *formulaParser) parseComparison() (formulaValue, bool) {
	left, ok := p.parseConcat()
	if !ok {
		return formulaValue{}, false
	}
	for {
		op := p.peek().text
		if p.peek().typ != formulaTokenSymbol || (op != "=" && op != "!=" && op != "<>" && op != "<" && op != ">" && op != "<=" && op != ">=") {
			return left, true
		}
		p.pos++
		right, ok := p.parseConcat()
		if !ok {
			return formulaValue{}, false
		}
		left = formulaValue{kind: formulaBool, bool: compareFormulaValues(left, right, op)}
	}
}

func (p *formulaParser) parseConcat() (formulaValue, bool) {
	left, ok := p.parseUnary()
	if !ok {
		return formulaValue{}, false
	}
	for p.matchSymbol("&") {
		right, ok := p.parseUnary()
		if !ok {
			return formulaValue{}, false
		}
		left = formulaValue{kind: formulaString, text: left.asString() + right.asString()}
	}
	return left, true
}

func (p *formulaParser) parseUnary() (formulaValue, bool) {
	if p.matchSymbol("!") {
		value, ok := p.parseUnary()
		if !ok {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: !value.truthy()}, true
	}
	return p.parsePrimary()
}

func (p *formulaParser) parsePrimary() (formulaValue, bool) {
	token := p.peek()
	switch token.typ {
	case formulaTokenString:
		p.pos++
		return formulaValue{kind: formulaString, text: token.text}, true
	case formulaTokenNumber:
		p.pos++
		number, err := strconv.ParseFloat(token.text, 64)
		if err != nil {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaNumber, number: number}, true
	case formulaTokenIdent:
		p.pos++
		if p.matchSymbol("(") {
			args, ok := p.parseArguments()
			if !ok {
				return formulaValue{}, false
			}
			return evaluateFormulaFunction(token.text, args)
		}
		switch strings.ToUpper(token.text) {
		case "TRUE":
			return formulaValue{kind: formulaBool, bool: true}, true
		case "FALSE":
			return formulaValue{kind: formulaBool, bool: false}, true
		case "NULL":
			return formulaValue{kind: formulaNull}, true
		}
		return formulaFieldValue(p.record, token.text), true
	case formulaTokenSymbol:
		if p.matchSymbol("(") {
			value, ok := p.parseExpression()
			if !ok || !p.matchSymbol(")") {
				return formulaValue{}, false
			}
			return value, true
		}
	}
	return formulaValue{}, false
}

func (p *formulaParser) parseArguments() ([]formulaValue, bool) {
	args := []formulaValue(nil)
	if p.matchSymbol(")") {
		return args, true
	}
	for {
		arg, ok := p.parseExpression()
		if !ok {
			return nil, false
		}
		args = append(args, arg)
		if p.matchSymbol(")") {
			return args, true
		}
		if !p.matchSymbol(",") {
			return nil, false
		}
	}
}

func (p *formulaParser) peek() formulaToken {
	if p.pos >= len(p.tokens) {
		return formulaToken{typ: formulaTokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *formulaParser) matchSymbol(symbol string) bool {
	if p.peek().typ == formulaTokenSymbol && p.peek().text == symbol {
		p.pos++
		return true
	}
	return false
}

func evaluateFormulaFunction(name string, args []formulaValue) (formulaValue, bool) {
	switch strings.ToUpper(name) {
	case "AND":
		for _, arg := range args {
			if !arg.truthy() {
				return formulaValue{kind: formulaBool, bool: false}, true
			}
		}
		return formulaValue{kind: formulaBool, bool: true}, true
	case "OR":
		for _, arg := range args {
			if arg.truthy() {
				return formulaValue{kind: formulaBool, bool: true}, true
			}
		}
		return formulaValue{kind: formulaBool, bool: false}, true
	case "NOT":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: !args[0].truthy()}, true
	case "ISBLANK", "ISNULL":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: args[0].blank()}, true
	case "CONTAINS":
		if len(args) != 2 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: strings.Contains(args[0].asString(), args[1].asString())}, true
	case "REGEX":
		if len(args) != 2 {
			return formulaValue{}, false
		}
		matches, err := regexp.MatchString(args[1].asString(), args[0].asString())
		if err != nil {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: matches}, true
	case "ISPICKVAL":
		if len(args) != 2 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: args[0].asString() == args[1].asString()}, true
	case "TEXT":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaString, text: args[0].asString()}, true
	case "LEN":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaNumber, number: float64(len(args[0].asString()))}, true
	default:
		return formulaValue{}, false
	}
}

func formulaFieldValue(record storage.Record, field string) formulaValue {
	if field == "Id" {
		if record.ID == "" {
			return formulaValue{kind: formulaNull}
		}
		return formulaValue{kind: formulaString, text: string(record.ID)}
	}
	if record.ExplicitNulls[field] {
		return formulaValue{kind: formulaNull}
	}
	value, ok := record.Fields[field]
	if !ok || value.Kind == storage.ValueNull {
		return formulaValue{kind: formulaNull}
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return formulaValue{kind: formulaString, text: value.String}
	case storage.ValueDecimal:
		number, err := strconv.ParseFloat(value.Decimal, 64)
		if err != nil {
			return formulaValue{kind: formulaString, text: value.Decimal}
		}
		return formulaValue{kind: formulaNumber, number: number}
	case storage.ValueID:
		return formulaValue{kind: formulaString, text: string(value.ID)}
	case storage.ValueInteger:
		return formulaValue{kind: formulaNumber, number: float64(value.Integer)}
	case storage.ValueBoolean:
		return formulaValue{kind: formulaBool, bool: value.Boolean}
	default:
		return formulaValue{kind: formulaString, text: fmt.Sprint(value)}
	}
}

func validationFieldBlank(record storage.Record, field string) bool {
	return formulaFieldValue(record, field).blank()
}

func validationFieldEquals(record storage.Record, field, want string) bool {
	return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, "=")
}

func compareFormulaValues(left, right formulaValue, op string) bool {
	if (left.kind == formulaNumber || right.kind == formulaNumber) && left.kind != formulaNull && right.kind != formulaNull {
		ln, lok := left.asNumber()
		rn, rok := right.asNumber()
		if lok && rok {
			switch op {
			case "=":
				return ln == rn
			case "!=", "<>":
				return ln != rn
			case "<":
				return ln < rn
			case ">":
				return ln > rn
			case "<=":
				return ln <= rn
			case ">=":
				return ln >= rn
			}
		}
	}
	if left.kind == formulaBool || right.kind == formulaBool {
		lb := left.truthy()
		rb := right.truthy()
		switch op {
		case "=":
			return lb == rb
		case "!=", "<>":
			return lb != rb
		default:
			return false
		}
	}
	ls := left.asString()
	rs := right.asString()
	switch op {
	case "=":
		return ls == rs
	case "!=", "<>":
		return ls != rs
	case "<":
		return ls < rs
	case ">":
		return ls > rs
	case "<=":
		return ls <= rs
	case ">=":
		return ls >= rs
	default:
		return false
	}
}

func (v formulaValue) truthy() bool {
	switch v.kind {
	case formulaBool:
		return v.bool
	case formulaNumber:
		return v.number != 0
	case formulaString:
		return v.text != ""
	default:
		return false
	}
}

func (v formulaValue) blank() bool {
	return v.kind == formulaNull || (v.kind == formulaString && v.text == "")
}

func (v formulaValue) asString() string {
	switch v.kind {
	case formulaBool:
		if v.bool {
			return "true"
		}
		return "false"
	case formulaNumber:
		return strconv.FormatFloat(v.number, 'f', -1, 64)
	case formulaString:
		return v.text
	default:
		return ""
	}
}

func (v formulaValue) asNumber() (float64, bool) {
	switch v.kind {
	case formulaNumber:
		return v.number, true
	case formulaString:
		number, err := strconv.ParseFloat(v.text, 64)
		return number, err == nil
	default:
		return 0, false
	}
}
