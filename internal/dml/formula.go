package dml

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/storage"
)

type formulaKind int

const (
	formulaNull formulaKind = iota
	formulaBool
	formulaString
	formulaNumber
	formulaDate
)

type formulaValue struct {
	kind      formulaKind
	bool      bool
	text      string
	number    float64
	fieldType storage.FieldType
	display   string
}

func evaluateRecordFormula(formula string, record storage.Record) (bool, bool) {
	parser := formulaParser{tokens: tokenizeFormula(html.UnescapeString(formula)), record: record}
	value, ok := parser.parseExpression()
	if !ok || parser.peek().typ != formulaTokenEOF {
		return false, false
	}
	return value.truthy(), true
}

func evaluateRecordFormulaInOrg(formula string, org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record) (bool, bool) {
	parser := formulaParser{
		tokens:     tokenizeFormula(html.UnescapeString(formula)),
		record:     record,
		org:        org,
		definition: definition,
	}
	value, ok := parser.parseExpression()
	if !ok || parser.peek().typ != formulaTokenEOF {
		return false, false
	}
	return value.truthy(), true
}

func evaluateRecordFormulaInOrgWithContext(formula string, org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, prior *storage.Record, isNew bool) (bool, bool) {
	parser := formulaParser{
		tokens:     tokenizeFormula(html.UnescapeString(formula)),
		record:     record,
		org:        org,
		definition: definition,
		prior:      prior,
		isNew:      isNew,
	}
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

func EvaluateRecordFormulaValueInOrg(formula string, field storage.Field, org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record) (storage.Value, bool, bool) {
	parser := formulaParser{
		tokens:     tokenizeFormula(html.UnescapeString(formula)),
		record:     record,
		org:        org,
		definition: definition,
		evaluating: make(map[string]bool),
	}
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
			if pair == "&&" || pair == "||" || pair == "==" || pair == "!=" || pair == "<>" || pair == "<=" || pair == ">=" {
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
	tokens     []formulaToken
	pos        int
	record     storage.Record
	org        *storage.OrgState
	definition storage.ObjectDefinition
	evaluating map[string]bool
	prior      *storage.Record
	isNew      bool
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
		if p.peek().typ != formulaTokenSymbol || (op != "=" && op != "==" && op != "!=" && op != "<>" && op != "<" && op != ">" && op != "<=" && op != ">=") {
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
	left, ok := p.parseAdditive()
	if !ok {
		return formulaValue{}, false
	}
	for p.matchSymbol("&") {
		right, ok := p.parseAdditive()
		if !ok {
			return formulaValue{}, false
		}
		left = formulaValue{kind: formulaString, text: left.asString() + right.asString()}
	}
	return left, true
}

func (p *formulaParser) parseAdditive() (formulaValue, bool) {
	left, ok := p.parseMultiplicative()
	if !ok {
		return formulaValue{}, false
	}
	for {
		op := p.peek().text
		if p.peek().typ != formulaTokenSymbol || (op != "+" && op != "-") {
			return left, true
		}
		p.pos++
		right, ok := p.parseMultiplicative()
		if !ok {
			return formulaValue{}, false
		}
		leftNumber, leftOK := left.asNumber()
		rightNumber, rightOK := right.asNumber()
		if (left.kind == formulaDate || looksLikeFormulaDate(left.asString())) && rightOK {
			if updated, ok := formulaDateAddDays(left.asString(), rightNumber, op); ok {
				left = updated
				continue
			}
		}
		if !leftOK || !rightOK {
			return formulaValue{}, false
		}
		if op == "+" {
			left = formulaValue{kind: formulaNumber, number: leftNumber + rightNumber}
		} else {
			left = formulaValue{kind: formulaNumber, number: leftNumber - rightNumber}
		}
	}
}

func (p *formulaParser) parseMultiplicative() (formulaValue, bool) {
	left, ok := p.parseUnary()
	if !ok {
		return formulaValue{}, false
	}
	for {
		op := p.peek().text
		if p.peek().typ != formulaTokenSymbol || (op != "*" && op != "/") {
			return left, true
		}
		p.pos++
		right, ok := p.parseUnary()
		if !ok {
			return formulaValue{}, false
		}
		leftNumber, leftOK := left.asNumber()
		rightNumber, rightOK := right.asNumber()
		if !leftOK || !rightOK {
			return formulaValue{}, false
		}
		if op == "*" {
			left = formulaValue{kind: formulaNumber, number: leftNumber * rightNumber}
			continue
		}
		if rightNumber == 0 {
			return formulaValue{kind: formulaNull}, true
		}
		left = formulaValue{kind: formulaNumber, number: leftNumber / rightNumber}
	}
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
			if strings.EqualFold(token.text, "IF") {
				return p.parseIfFunction()
			}
			if strings.EqualFold(token.text, "PRIORVALUE") {
				return p.parsePriorValueFunction()
			}
			args, ok := p.parseArguments()
			if !ok {
				return formulaValue{}, false
			}
			return p.evaluateFormulaFunction(token.text, args)
		}
		switch strings.ToUpper(token.text) {
		case "TRUE":
			return formulaValue{kind: formulaBool, bool: true}, true
		case "FALSE":
			return formulaValue{kind: formulaBool, bool: false}, true
		case "NULL":
			return formulaValue{kind: formulaNull}, true
		}
		return p.fieldValue(token.text), true
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

func (p *formulaParser) parseIfFunction() (formulaValue, bool) {
	condition, ok := p.parseExpression()
	if !ok || !p.matchSymbol(",") {
		return formulaValue{}, false
	}
	if condition.truthy() {
		value, ok := p.parseExpression()
		if !ok {
			return formulaValue{}, false
		}
		if p.matchSymbol(",") {
			if !p.skipFormulaArgument() {
				return formulaValue{}, false
			}
		}
		if !p.matchSymbol(")") {
			return formulaValue{}, false
		}
		return value, true
	}
	if !p.skipFormulaArgument() || !p.matchSymbol(",") {
		return formulaValue{}, false
	}
	value, ok := p.parseExpression()
	if !ok || !p.matchSymbol(")") {
		return formulaValue{}, false
	}
	return value, true
}

func (p *formulaParser) parsePriorValueFunction() (formulaValue, bool) {
	token := p.peek()
	if token.typ != formulaTokenIdent {
		return formulaValue{}, false
	}
	p.pos++
	if !p.matchSymbol(")") {
		return formulaValue{}, false
	}
	if p.prior == nil {
		return p.valueForRecordField(p.record, token.text), true
	}
	return p.valueForRecordField(*p.prior, token.text), true
}

func (p *formulaParser) skipFormulaArgument() bool {
	depth := 0
	for {
		token := p.peek()
		if token.typ == formulaTokenEOF {
			return false
		}
		if token.typ == formulaTokenSymbol {
			switch token.text {
			case "(":
				depth++
			case ")":
				if depth == 0 {
					return true
				}
				depth--
			case ",":
				if depth == 0 {
					return true
				}
			}
		}
		p.pos++
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

func (p *formulaParser) fieldValue(field string) formulaValue {
	return p.valueForRecordField(p.record, field)
}

func (p *formulaParser) valueForRecordField(record storage.Record, field string) formulaValue {
	if p.org == nil {
		return formulaFieldValue(record, field)
	}
	if field == "$RecordType.Name" || field == "$RecordType.DeveloperName" {
		return formulaValue{kind: formulaString, text: formulaRecordTypeValue(p.definition, record, field)}
	}
	if value, ok := p.setupFieldValue(field); ok {
		return value
	}
	if strings.Contains(field, ".") {
		if value, ok := formulaRelationshipFieldValue(p.org, p.definition, record, field, p.evaluating); ok {
			return value
		}
	}
	if resolved, ok := storage.ResolveFieldName(p.definition, p.org.Namespace, field); ok {
		field = resolved
	}
	if p.definition.APIName != "" {
		if fieldDef, ok := p.definition.Fields[field]; ok {
			if fieldDef.Type == storage.FieldSummary {
				engine := Engine{Org: p.org}
				if value, ok := engine.evaluateSummaryField(p.record, fieldDef); ok {
					return formulaStorageValue(value)
				}
			}
			if strings.TrimSpace(fieldDef.Formula) != "" {
				if p.evaluating == nil {
					p.evaluating = make(map[string]bool)
				}
				if p.evaluating[field] {
					return formulaValue{kind: formulaNull}
				}
				p.evaluating[field] = true
				nested := formulaParser{
					tokens:     tokenizeFormula(html.UnescapeString(fieldDef.Formula)),
					record:     record,
					org:        p.org,
					definition: p.definition,
					evaluating: p.evaluating,
					prior:      p.prior,
					isNew:      p.isNew,
				}
				value, ok := nested.parseExpression()
				delete(p.evaluating, field)
				if ok && nested.peek().typ == formulaTokenEOF {
					return value
				}
			}
		}
	}
	if p.definition.APIName != "" {
		if fieldDef, ok := p.definition.Fields[field]; ok {
			return formulaFieldValueForDefinition(record, field, fieldDef)
		}
	}
	return formulaFieldValue(record, field)
}

func (p *formulaParser) setupFieldValue(field string) (formulaValue, bool) {
	if p.org == nil || !strings.HasPrefix(field, "$Setup.") {
		return formulaValue{}, false
	}
	parts := strings.Split(field, ".")
	if len(parts) != 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return formulaValue{kind: formulaNull}, true
	}
	objectName, ok := storage.ResolveObjectName(*p.org, parts[1])
	if !ok {
		return formulaValue{kind: formulaNull}, true
	}
	object := p.org.Objects[objectName]
	definition := object.Definition
	if !storage.IsCustomSettingDefinition(definition) {
		return formulaValue{kind: formulaNull}, true
	}
	fieldName := parts[2]
	if resolved, ok := storage.ResolveFieldName(definition, p.org.Namespace, fieldName); ok {
		fieldName = resolved
	}
	fieldDef, hasField := definition.Fields[fieldName]
	if !hasField {
		return formulaValue{kind: formulaNull}, true
	}
	record, found := setupCustomSettingRecord(*p.org, object)
	if found {
		return formulaFieldValueForDefinition(record, fieldName, fieldDef), true
	}
	if value, ok := defaultValueForRecordField(p.org, definition, storage.Record{Object: objectName, Fields: map[string]storage.Value{}}, fieldDef); ok {
		return formulaStorageValueForDefinition(value, fieldDef), true
	}
	return formulaValue{kind: formulaNull, fieldType: fieldDef.Type, display: strings.ToUpper(fieldDef.DisplayType)}, true
}

func setupCustomSettingRecord(org storage.OrgState, object storage.ObjectState) (storage.Record, bool) {
	orgID := strings.TrimSpace(org.OrgID)
	if orgID == "" {
		orgID = "00D000000000001"
	}
	records := make([]storage.Record, 0, len(object.Records))
	for _, record := range object.Records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		return string(records[i].ID) < string(records[j].ID)
	})
	for _, record := range records {
		if record.System.IsDeleted {
			continue
		}
		if strings.EqualFold(object.Definition.Metadata["customSettingsType"], "Hierarchy") {
			if value, ok := record.GetField("SetupOwnerId"); ok && value.Kind == storage.ValueString && strings.TrimSpace(value.String) == orgID {
				return record, true
			}
			if value, ok := record.GetField("Name"); ok && value.Kind == storage.ValueString && strings.TrimSpace(value.String) == orgID {
				return record, true
			}
			continue
		}
		return record, true
	}
	return storage.Record{}, false
}

func formulaRecordTypeValue(definition storage.ObjectDefinition, record storage.Record, field string) string {
	recordTypeID := ""
	if value, ok := record.GetField("RecordTypeId"); ok {
		recordTypeID = strings.TrimSpace(workflowValueString(value))
	}
	for _, recordType := range definition.RecordTypes {
		if recordTypeID != "" && string(recordType.ID) != recordTypeID {
			continue
		}
		if field == "$RecordType.DeveloperName" {
			return recordType.DeveloperName
		}
		if recordType.Name != "" {
			return recordType.Name
		}
		return recordType.DeveloperName
	}
	return ""
}

func (p *formulaParser) evaluateFormulaFunction(name string, args []formulaValue) (formulaValue, bool) {
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
	case "ISNEW":
		if len(args) != 0 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: p.isNew}, true
	case "ISBLANK":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: args[0].blank()}, true
	case "BLANKVALUE":
		if len(args) != 2 {
			return formulaValue{}, false
		}
		if args[0].blank() {
			return args[1], true
		}
		return args[0], true
	case "ISNULL":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: args[0].isNull()}, true
	case "CONTAINS":
		if len(args) != 2 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaBool, bool: strings.Contains(args[0].asString(), args[1].asString())}, true
	case "CASE":
		if len(args) < 4 {
			return formulaValue{}, false
		}
		probe := args[0]
		hasDefault := len(args)%2 == 0
		end := len(args)
		if hasDefault {
			end--
		}
		for i := 1; i+1 < end; i += 2 {
			if compareFormulaValues(probe, args[i], "=") {
				return args[i+1], true
			}
		}
		if hasDefault {
			return args[len(args)-1], true
		}
		return formulaValue{kind: formulaNull}, true
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
	case "LOWER":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		if args[0].isNull() {
			return formulaValue{kind: formulaNull}, true
		}
		return formulaValue{kind: formulaString, text: strings.ToLower(args[0].asString())}, true
	case "UPPER":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		if args[0].isNull() {
			return formulaValue{kind: formulaNull}, true
		}
		return formulaValue{kind: formulaString, text: strings.ToUpper(args[0].asString())}, true
	case "TODAY":
		if len(args) != 0 {
			return formulaValue{}, false
		}
		now := time.Now().UTC()
		if p.org != nil && p.org.Now != nil {
			now = p.org.Now().UTC()
		}
		return formulaValue{kind: formulaDate, text: now.Format("2006-01-02")}, true
	case "DATE":
		if len(args) != 3 {
			return formulaValue{}, false
		}
		year, ok := formulaIntArg(args[0])
		if !ok {
			return formulaValue{}, false
		}
		month, ok := formulaIntArg(args[1])
		if !ok {
			return formulaValue{}, false
		}
		day, ok := formulaIntArg(args[2])
		if !ok {
			return formulaValue{}, false
		}
		date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		return formulaValue{kind: formulaDate, text: date.Format("2006-01-02")}, true
	case "DAY", "MONTH", "YEAR":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		date, ok := parseFormulaDate(args[0].asString())
		if !ok {
			return formulaValue{}, false
		}
		switch strings.ToUpper(name) {
		case "DAY":
			return formulaValue{kind: formulaNumber, number: float64(date.Day())}, true
		case "MONTH":
			return formulaValue{kind: formulaNumber, number: float64(int(date.Month()))}, true
		default:
			return formulaValue{kind: formulaNumber, number: float64(date.Year())}, true
		}
	case "FLOOR":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		number, ok := args[0].asNumber()
		if !ok {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaNumber, number: math.Floor(number)}, true
	case "ABS":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		number, ok := args[0].asNumber()
		if !ok {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaNumber, number: math.Abs(number)}, true
	case "MOD":
		if len(args) != 2 {
			return formulaValue{}, false
		}
		dividend, ok := args[0].asNumber()
		if !ok {
			return formulaValue{}, false
		}
		divisor, ok := args[1].asNumber()
		if !ok {
			return formulaValue{}, false
		}
		if divisor == 0 {
			return formulaValue{kind: formulaNull}, true
		}
		return formulaValue{kind: formulaNumber, number: math.Mod(dividend, divisor)}, true
	case "LEN":
		if len(args) != 1 {
			return formulaValue{}, false
		}
		return formulaValue{kind: formulaNumber, number: float64(len(args[0].asString()))}, true
	default:
		return formulaValue{}, false
	}
}

func formulaRelationshipFieldValue(org *storage.OrgState, definition storage.ObjectDefinition, record storage.Record, fieldPath string, evaluating map[string]bool) (formulaValue, bool) {
	parts := strings.Split(fieldPath, ".")
	if org == nil || len(parts) < 2 {
		return formulaValue{}, false
	}
	currentDefinition := definition
	currentRecord := record
	for _, relationship := range parts[:len(parts)-1] {
		lookupField, ok := relationshipLookupField(currentDefinition, org.Namespace, relationship)
		if !ok {
			return formulaValue{}, false
		}
		value, ok := formulaRecordField(currentRecord, lookupField.APIName)
		if !ok || value.Kind == storage.ValueNull {
			return formulaValue{kind: formulaNull}, true
		}
		parentID := idFromStorageValue(value)
		if parentID == "" {
			return formulaValue{kind: formulaNull}, true
		}
		parentRecord, parentDefinition, ok := formulaParentRecord(*org, lookupField, parentID)
		if !ok {
			return formulaValue{kind: formulaNull}, true
		}
		currentRecord = parentRecord
		currentDefinition = parentDefinition
	}
	last := parts[len(parts)-1]
	if resolved, ok := storage.ResolveFieldName(currentDefinition, org.Namespace, last); ok {
		last = resolved
	}
	if fieldDef, ok := currentDefinition.Fields[last]; ok {
		if fieldDef.Type == storage.FieldSummary {
			engine := Engine{Org: org}
			if value, ok := engine.evaluateSummaryField(currentRecord, fieldDef); ok {
				return formulaStorageValue(value), true
			}
		}
		if strings.TrimSpace(fieldDef.Formula) != "" {
			if evaluating == nil {
				evaluating = make(map[string]bool)
			}
			key := currentDefinition.APIName + "." + string(currentRecord.ID) + "." + last
			if evaluating[key] {
				return formulaValue{kind: formulaNull}, true
			}
			evaluating[key] = true
			nested := formulaParser{
				tokens:     tokenizeFormula(html.UnescapeString(fieldDef.Formula)),
				record:     currentRecord,
				org:        org,
				definition: currentDefinition,
				evaluating: evaluating,
			}
			value, ok := nested.parseExpression()
			delete(evaluating, key)
			if ok && nested.peek().typ == formulaTokenEOF {
				return value, true
			}
		}
	}
	if fieldDef, ok := currentDefinition.Fields[last]; ok {
		if value, ok := formulaRecordField(currentRecord, last); ok && value.Kind != storage.ValueNull {
			return formulaStorageValueForDefinition(value, fieldDef), true
		}
	}
	if fieldDef, ok := currentDefinition.Fields[last]; ok && strings.TrimSpace(fieldDef.DefaultValue) != "" {
		if value, ok := defaultValueForRecordField(org, currentDefinition, currentRecord, fieldDef); ok {
			return formulaStorageValueForDefinition(value, fieldDef), true
		}
	}
	if fieldDef, ok := currentDefinition.Fields[last]; ok {
		return formulaFieldValueForDefinition(currentRecord, last, fieldDef), true
	}
	return formulaFieldValue(currentRecord, last), true
}

func relationshipLookupField(definition storage.ObjectDefinition, namespace, relationship string) (storage.Field, bool) {
	for _, lookupName := range relationshipLookupCandidates(relationship) {
		if canonical, ok := storage.ResolveFieldName(definition, namespace, lookupName); ok {
			field := definition.Fields[canonical]
			if field.APIName == "" {
				field.APIName = canonical
			}
			if len(field.ReferenceTo) > 0 {
				return field, true
			}
		}
	}
	names := make([]string, 0, len(definition.Fields))
	for name := range definition.Fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		field := definition.Fields[name]
		if field.APIName == "" {
			field.APIName = name
		}
		if len(field.ReferenceTo) == 0 {
			continue
		}
		if relationshipNameMatches(namespace, field, relationship) {
			return field, true
		}
	}
	return storage.Field{}, false
}

func relationshipLookupCandidates(relationship string) []string {
	relationship = strings.TrimSpace(relationship)
	if relationship == "" {
		return nil
	}
	candidates := []string(nil)
	if strings.HasSuffix(relationship, "__r") {
		candidates = append(candidates, strings.TrimSuffix(relationship, "__r")+"__c")
	}
	if !strings.HasSuffix(relationship, "__r") && !strings.HasSuffix(relationship, "Id") {
		candidates = append(candidates, relationship+"Id")
	}
	return candidates
}

func relationshipNameMatches(namespace string, field storage.Field, relationship string) bool {
	for _, candidate := range formulaRelationshipNames(field) {
		if strings.EqualFold(candidate, relationship) ||
			strings.EqualFold(storage.NamespaceTokenName(namespace, candidate), relationship) ||
			strings.EqualFold(storage.StripNamespaceToken(namespace, candidate), relationship) {
			return true
		}
	}
	return false
}

func formulaRelationshipNames(field storage.Field) []string {
	names := []string(nil)
	if field.RelationshipName != "" {
		names = append(names, field.RelationshipName)
	}
	apiName := field.APIName
	switch {
	case strings.HasSuffix(apiName, "__c"):
		names = append(names, strings.TrimSuffix(apiName, "__c")+"__r")
	case strings.HasSuffix(apiName, "Id") && len(apiName) > len("Id"):
		names = append(names, strings.TrimSuffix(apiName, "Id"))
	}
	return names
}

func formulaParentRecord(org storage.OrgState, lookupField storage.Field, id storage.ID) (storage.Record, storage.ObjectDefinition, bool) {
	for _, targetName := range lookupField.ReferenceTo {
		canonical, ok := storage.ResolveObjectName(org, targetName)
		if !ok {
			continue
		}
		target := org.Objects[canonical]
		record, ok := target.Records[id]
		if ok && !record.System.IsDeleted {
			return record, target.Definition, true
		}
	}
	return storage.Record{}, storage.ObjectDefinition{}, false
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
	value, ok := formulaRecordField(record, field)
	if !ok || value.Kind == storage.ValueNull {
		return formulaValue{kind: formulaNull}
	}
	return formulaStorageValue(value)
}

func formulaFieldValueForDefinition(record storage.Record, field string, fieldDef storage.Field) formulaValue {
	if field == "Id" {
		value := formulaFieldValue(record, field)
		value.fieldType = fieldDef.Type
		value.display = strings.ToUpper(fieldDef.DisplayType)
		return value
	}
	if record.ExplicitNulls[field] {
		return formulaValue{kind: formulaNull, fieldType: fieldDef.Type, display: strings.ToUpper(fieldDef.DisplayType)}
	}
	value, ok := formulaRecordField(record, field)
	if !ok || value.Kind == storage.ValueNull {
		return formulaValue{kind: formulaNull, fieldType: fieldDef.Type, display: strings.ToUpper(fieldDef.DisplayType)}
	}
	return formulaStorageValueForDefinition(value, fieldDef)
}

func formulaRecordField(record storage.Record, field string) (storage.Value, bool) {
	stripped := storage.StripAnyNamespaceToken(field)
	if stripped != field {
		if value, ok := record.GetField(stripped); ok {
			return value, true
		}
	}
	if value, ok := record.GetField(field); ok {
		return value, true
	}
	for candidate, value := range record.Fields {
		if strings.EqualFold(storage.StripAnyNamespaceToken(candidate), stripped) {
			return value, true
		}
	}
	return storage.Value{}, false
}

func formulaStorageValue(value storage.Value) formulaValue {
	if value.Kind == storage.ValueNull {
		return formulaValue{kind: formulaNull}
	}
	switch value.Kind {
	case storage.ValueString, storage.ValueDateTime:
		return formulaValue{kind: formulaString, text: value.String}
	case storage.ValueDate:
		return formulaValue{kind: formulaDate, text: value.String}
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

func formulaStorageValueForDefinition(value storage.Value, fieldDef storage.Field) formulaValue {
	out := formulaStorageValue(value)
	out.fieldType = fieldDef.Type
	out.display = strings.ToUpper(fieldDef.DisplayType)
	if out.kind == formulaNumber && out.display == "PERCENT" {
		out.number = out.number / 100
	}
	return out
}

func validationFieldBlank(record storage.Record, field string) bool {
	return formulaFieldValue(record, field).blank()
}

func validationFieldEquals(record storage.Record, field, want string) bool {
	return compareFormulaValues(formulaFieldValue(record, field), formulaValue{kind: formulaString, text: want}, "=")
}

func compareFormulaValues(left, right formulaValue, op string) bool {
	if left.kind == formulaNull || right.kind == formulaNull {
		switch op {
		case "=":
			return left.kind == formulaNull && right.kind == formulaNull
		case "!=", "<>":
			return left.kind != right.kind
		default:
			return false
		}
	}
	if (left.kind == formulaNumber || right.kind == formulaNumber) && left.kind != formulaNull && right.kind != formulaNull {
		ln, lok := left.asNumber()
		rn, rok := right.asNumber()
		if lok && rok {
			switch op {
			case "=", "==":
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
		case "=", "==":
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
	case "=", "==":
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

func (v formulaValue) isNull() bool {
	if v.fieldType == storage.FieldString || v.fieldType == storage.FieldReference {
		return false
	}
	return v.kind == formulaNull
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
	case formulaDate:
		return v.text
	default:
		return ""
	}
}

func (v formulaValue) asNumber() (float64, bool) {
	switch v.kind {
	case formulaNull:
		return 0, true
	case formulaNumber:
		return v.number, true
	case formulaString:
		number, err := strconv.ParseFloat(v.text, 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func formulaIntArg(value formulaValue) (int, bool) {
	number, ok := value.asNumber()
	if !ok {
		return 0, false
	}
	return int(number), true
}

func formulaDateAddDays(dateText string, days float64, op string) (formulaValue, bool) {
	date, ok := parseFormulaDate(dateText)
	if !ok {
		return formulaValue{}, false
	}
	if op == "-" {
		days = -days
	}
	return formulaValue{kind: formulaDate, text: date.AddDate(0, 0, int(days)).Format("2006-01-02")}, true
}

func parseFormulaDate(text string) (time.Time, bool) {
	text = strings.TrimSpace(text)
	if len(text) >= len("2006-01-02") {
		text = text[:len("2006-01-02")]
	}
	date, err := time.Parse("2006-01-02", text)
	if err != nil {
		return time.Time{}, false
	}
	return date, true
}

func looksLikeFormulaDate(text string) bool {
	_, ok := parseFormulaDate(text)
	return ok
}
