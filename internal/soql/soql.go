package soql

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/open-aer/oaer/internal/storage"
)

type Query struct {
	Fields  []string
	Object  string
	Where   *Condition
	OrderBy string
	Limit   int
	Offset  int
	Count   bool
}

type Condition struct {
	Field string
	Op    string
	Value storage.Value
}

type Result struct {
	Records []storage.Record `json:"records"`
	Rows    int              `json:"rows"`
}

func Parse(input string) (Query, error) {
	tokens, err := lex(input)
	if err != nil {
		return Query{}, err
	}
	p := parser{tokens: tokens}
	return p.parseQuery()
}

func Execute(org storage.OrgState, query Query) (Result, error) {
	object, ok := org.Objects[query.Object]
	if !ok {
		return Result{}, fmt.Errorf("soql: unknown object %s", query.Object)
	}
	if len(query.Fields) == 0 {
		return Result{}, fmt.Errorf("soql: SELECT requires at least one field")
	}

	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	records := make([]storage.Record, 0, len(ids))
	for _, idText := range ids {
		record := object.Records[storage.ID(idText)]
		if matches(record, query.Where) {
			records = append(records, projectRecord(org, record, query.Fields))
		}
	}
	if query.OrderBy != "" {
		sort.SliceStable(records, func(i, j int) bool {
			return valueSortKey(records[i].Fields[query.OrderBy]) < valueSortKey(records[j].Fields[query.OrderBy])
		})
	}
	if query.Offset > 0 {
		if query.Offset >= len(records) {
			records = nil
		} else {
			records = records[query.Offset:]
		}
	}
	if query.Limit > 0 && query.Limit < len(records) {
		records = records[:query.Limit]
	}
	if query.Count {
		return Result{
			Records: []storage.Record{{
				Object: "AggregateResult",
				Fields: map[string]storage.Value{
					"expr0": storage.IntegerValue(int64(len(records))),
				},
			}},
			Rows: 1,
		}, nil
	}
	return Result{Records: records, Rows: len(records)}, nil
}

func ParseAndExecute(org storage.OrgState, input string) (Result, error) {
	query, err := Parse(input)
	if err != nil {
		return Result{}, err
	}
	return Execute(org, query)
}

func matches(record storage.Record, condition *Condition) bool {
	if condition == nil {
		return true
	}
	left, ok := recordValue(record, condition.Field)
	if !ok {
		left = storage.NullValue()
	}
	switch condition.Op {
	case "=":
		return equalValues(left, condition.Value)
	case "!=":
		return !equalValues(left, condition.Value)
	default:
		return false
	}
}

func projectRecord(org storage.OrgState, record storage.Record, fields []string) storage.Record {
	out := storage.Record{
		ID:            record.ID,
		Object:        record.Object,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
		System:        record.System,
	}
	for _, field := range fields {
		if field == "Id" {
			out.Fields[field] = storage.IDValue(record.ID)
			continue
		}
		if strings.Contains(field, ".") {
			if value, ok := relationshipValue(org, record, field); ok {
				out.Fields[field] = value
			}
			continue
		}
		if record.ExplicitNulls[field] {
			out.ExplicitNulls[field] = true
			continue
		}
		if value, ok := record.Fields[field]; ok {
			out.Fields[field] = value.Clone()
		}
	}
	return out
}

func relationshipValue(org storage.OrgState, record storage.Record, field string) (storage.Value, bool) {
	parts := strings.SplitN(field, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return storage.Value{}, false
	}
	object, ok := org.Objects[record.Object]
	if !ok {
		return storage.Value{}, false
	}
	for _, relation := range object.Definition.Relations {
		if relation.ParentRelationship != parts[0] {
			continue
		}
		parentID, ok := recordValue(record, relation.Field)
		if !ok || parentID.Kind == storage.ValueNull {
			return storage.NullValue(), true
		}
		for _, parentObjectName := range relation.ParentObjects {
			parentObject, ok := org.Objects[parentObjectName]
			if !ok {
				continue
			}
			parent, ok := parentObject.Records[idFromValue(parentID)]
			if !ok {
				continue
			}
			value, ok := recordValue(parent, parts[1])
			if !ok {
				return storage.NullValue(), true
			}
			return value.Clone(), true
		}
	}
	return storage.Value{}, false
}

func idFromValue(value storage.Value) storage.ID {
	if value.Kind == storage.ValueID {
		return value.ID
	}
	if value.Kind == storage.ValueString {
		return storage.ID(value.String)
	}
	return ""
}

func recordValue(record storage.Record, field string) (storage.Value, bool) {
	if field == "Id" {
		return storage.IDValue(record.ID), true
	}
	if record.ExplicitNulls[field] {
		return storage.NullValue(), true
	}
	value, ok := record.Fields[field]
	return value, ok
}

func equalValues(left, right storage.Value) bool {
	if left.Kind == storage.ValueID && right.Kind == storage.ValueString {
		return string(left.ID) == right.String
	}
	if left.Kind == storage.ValueString && right.Kind == storage.ValueID {
		return left.String == string(right.ID)
	}
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case storage.ValueNull:
		return true
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return left.String == right.String
	case storage.ValueInteger:
		return left.Integer == right.Integer
	case storage.ValueBoolean:
		return left.Boolean == right.Boolean
	case storage.ValueDecimal:
		return left.Decimal == right.Decimal
	case storage.ValueID:
		return left.ID == right.ID
	default:
		return false
	}
}

func valueSortKey(value storage.Value) string {
	switch value.Kind {
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime:
		return value.String
	case storage.ValueInteger:
		return fmt.Sprintf("%020d", value.Integer)
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueBoolean:
		if value.Boolean {
			return "1"
		}
		return "0"
	default:
		return ""
	}
}

type token struct {
	text string
}

func lex(input string) ([]token, error) {
	var out []token
	for i := 0; i < len(input); {
		switch {
		case input[i] == ' ' || input[i] == '\n' || input[i] == '\t' || input[i] == '\r':
			i++
		case input[i] == '\'':
			start := i
			i++
			for i < len(input) {
				if input[i] == '\'' {
					if i+1 < len(input) && input[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					out = append(out, token{text: input[start:i]})
					goto next
				}
				i++
			}
			return nil, fmt.Errorf("soql: unterminated string literal")
		case strings.ContainsRune(",=!*()", rune(input[i])):
			if i+1 < len(input) && input[i:i+2] == "!=" {
				out = append(out, token{text: "!="})
				i += 2
			} else {
				out = append(out, token{text: input[i : i+1]})
				i++
			}
		default:
			start := i
			for i < len(input) && !strings.ContainsRune(" \n\t\r,=!*()", rune(input[i])) {
				i++
			}
			out = append(out, token{text: input[start:i]})
		}
	next:
	}
	out = append(out, token{text: ""})
	return out, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) parseQuery() (Query, error) {
	if !p.matchWord("SELECT") {
		return Query{}, p.errorf("expected SELECT")
	}
	fields, err := p.parseFields()
	if err != nil {
		return Query{}, err
	}
	if !p.matchWord("FROM") {
		return Query{}, p.errorf("expected FROM")
	}
	object := p.advance().text
	if object == "" {
		return Query{}, p.errorf("expected object name")
	}
	q := Query{Fields: fields, Object: object, Count: len(fields) == 1 && strings.EqualFold(fields[0], "COUNT()")}
	for p.peek().text != "" {
		switch {
		case p.matchWord("WHERE"):
			condition, err := p.parseCondition()
			if err != nil {
				return Query{}, err
			}
			q.Where = &condition
		case p.matchWord("ORDER"):
			if !p.matchWord("BY") {
				return Query{}, p.errorf("expected BY after ORDER")
			}
			orderBy, err := p.parseName()
			if err != nil {
				return Query{}, err
			}
			q.OrderBy = orderBy
			if q.OrderBy == "" {
				return Query{}, p.errorf("expected ORDER BY field")
			}
		case p.matchWord("LIMIT"):
			limit, err := p.parseInt()
			if err != nil {
				return Query{}, err
			}
			q.Limit = limit
		case p.matchWord("OFFSET"):
			offset, err := p.parseInt()
			if err != nil {
				return Query{}, err
			}
			q.Offset = offset
		default:
			return Query{}, p.errorf("unsupported SOQL token %q", p.peek().text)
		}
	}
	return q, nil
}

func (p *parser) parseFields() ([]string, error) {
	var fields []string
	for {
		field, err := p.parseName()
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(field, "COUNT") && p.match("(") {
			if !p.match(")") {
				return nil, p.errorf("expected ) after COUNT(")
			}
			field = "COUNT()"
		}
		fields = append(fields, field)
		if !p.match(",") {
			return fields, nil
		}
	}
}

func (p *parser) parseCondition() (Condition, error) {
	field, err := p.parseName()
	if err != nil {
		return Condition{}, err
	}
	op := p.advance().text
	if op != "=" && op != "!=" {
		return Condition{}, p.errorf("unsupported WHERE operator %q", op)
	}
	valueToken := p.advance().text
	if valueToken == "" {
		return Condition{}, p.errorf("expected WHERE value")
	}
	value, err := literal(valueToken)
	if err != nil {
		return Condition{}, err
	}
	return Condition{Field: field, Op: op, Value: value}, nil
}

func (p *parser) parseName() (string, error) {
	name := p.advance().text
	if name == "" {
		return "", p.errorf("expected name")
	}
	for p.match(".") {
		part := p.advance().text
		if part == "" {
			return "", p.errorf("expected name after .")
		}
		name += "." + part
	}
	return name, nil
}

func (p *parser) parseInt() (int, error) {
	text := p.advance().text
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 {
		return 0, p.errorf("expected non-negative integer")
	}
	return value, nil
}

func (p *parser) match(text string) bool {
	if p.peek().text != text {
		return false
	}
	p.pos++
	return true
}

func (p *parser) matchWord(text string) bool {
	if !strings.EqualFold(p.peek().text, text) {
		return false
	}
	p.pos++
	return true
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("soql: "+format, args...)
}

func literal(text string) (storage.Value, error) {
	switch {
	case strings.EqualFold(text, "null"):
		return storage.NullValue(), nil
	case strings.EqualFold(text, "true"):
		return storage.BooleanValue(true), nil
	case strings.EqualFold(text, "false"):
		return storage.BooleanValue(false), nil
	case strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'"):
		inner := strings.TrimSuffix(strings.TrimPrefix(text, "'"), "'")
		return storage.StringValue(strings.ReplaceAll(inner, "''", "'")), nil
	default:
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return storage.IDValue(storage.ID(text)), nil
		}
		return storage.IntegerValue(value), nil
	}
}
