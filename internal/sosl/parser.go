package sosl

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Window struct {
	Value    int
	HasValue bool
	Bind     string
}

type SearchScope string

const (
	SearchScopeAll   SearchScope = "ALL FIELDS"
	SearchScopeName  SearchScope = "NAME FIELDS"
	SearchScopeEmail SearchScope = "EMAIL FIELDS"
	SearchScopePhone SearchScope = "PHONE FIELDS"
)

type SearchTerm struct {
	Text   string
	Prefix bool
}

type SelectExpr struct {
	Field string
	Func  string
	Alias string
}

type Condition struct {
	Field       string
	Operator    string
	Value       string
	Values      []string
	ValueIsNull bool
	Bind        string
}

type OrderSpec struct {
	Field string
	Desc  bool
}

type ReturningObject struct {
	Object  string
	Fields  []SelectExpr
	Where   *Condition
	OrderBy []OrderSpec
	Limit   Window
	Offset  Window
}

type Query struct {
	Terms             []SearchTerm
	Scope             SearchScope
	Returning         []ReturningObject
	Limit             Window
	WithSnippet       bool
	SpellCorrection   *bool
	PricebookID       string
	PricebookIDBind   string
	DivisionBind      string
	DivisionSpecified bool
}

type UnsupportedFeatureError struct {
	Message string
}

func (err *UnsupportedFeatureError) Error() string {
	if err == nil {
		return "unsupported SOSL feature"
	}
	return err.Message
}

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenWord
	tokenString
	tokenNumber
	tokenComma
	tokenLParen
	tokenRParen
	tokenLBrace
	tokenRBrace
	tokenColon
	tokenEqual
	tokenNotEqual
	tokenInvalid
)

type token struct {
	kind tokenKind
	text string
}

func Parse(input string) (Query, error) {
	parser := parser{tokens: lex(input)}
	return parser.parse()
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) parse() (Query, error) {
	var query Query
	if err := p.expectKeyword("FIND"); err != nil {
		return Query{}, err
	}
	terms, err := p.parseTerms()
	if err != nil {
		return Query{}, err
	}
	query.Terms = terms
	query.Scope = SearchScopeAll
	if p.acceptKeyword("IN") {
		var scope SearchScope
		switch {
		case p.acceptKeyword("ALL"):
			scope = SearchScopeAll
		case p.acceptKeyword("NAME"):
			scope = SearchScopeName
		case p.acceptKeyword("EMAIL"):
			scope = SearchScopeEmail
		case p.acceptKeyword("PHONE"):
			scope = SearchScopePhone
		default:
			return Query{}, p.errorf("expected SOSL search scope")
		}
		if err := p.expectKeyword("FIELDS"); err != nil {
			return Query{}, err
		}
		query.Scope = scope
	}
	for p.acceptKeyword("WITH") {
		if err := p.parseWith(&query); err != nil {
			return Query{}, err
		}
	}
	if err := p.expectKeyword("RETURNING"); err != nil {
		return Query{}, err
	}
	if err := p.parseReturning(&query); err != nil {
		return Query{}, err
	}
	for p.peek().kind != tokenEOF {
		if p.accept(tokenComma) {
			return Query{}, p.errorf("unexpected comma after RETURNING clause")
		}
		if p.acceptKeyword("WITH") {
			if err := p.parseWith(&query); err != nil {
				return Query{}, err
			}
			continue
		}
		if p.acceptKeyword("LIMIT") {
			window, err := p.parseWindow("LIMIT")
			if err != nil {
				return Query{}, err
			}
			query.Limit = window
			continue
		}
		if p.acceptKeyword("USING") {
			kind := p.next()
			return Query{}, &UnsupportedFeatureError{Message: fmt.Sprintf("SOSL USING %s hosted search service", kind.text)}
		}
		if p.acceptKeyword("UPDATE") {
			kind := p.next()
			return Query{}, &UnsupportedFeatureError{Message: fmt.Sprintf("SOSL UPDATE %s hosted search analytics", kind.text)}
		}
		return Query{}, p.errorf("unexpected SOSL token %q", p.peek().text)
	}
	return query, nil
}

func (p *parser) parseTerms() ([]SearchTerm, error) {
	var raw []string
	switch p.peek().kind {
	case tokenLBrace:
		p.next()
		for p.peek().kind != tokenRBrace && p.peek().kind != tokenEOF {
			tok := p.next()
			if tok.kind != tokenWord && tok.kind != tokenString {
				return nil, p.errorf("expected SOSL search term")
			}
			raw = append(raw, strings.Fields(tok.text)...)
		}
		if !p.accept(tokenRBrace) {
			return nil, p.errorf("unterminated SOSL search term")
		}
	case tokenString:
		raw = strings.Fields(p.next().text)
	case tokenColon:
		p.next()
		bind := p.next()
		if bind.kind != tokenWord {
			return nil, p.errorf("expected FIND bind")
		}
		raw = []string{":" + bind.text}
	case tokenWord:
		raw = []string{p.next().text}
	default:
		return nil, p.errorf("expected FIND search term")
	}
	terms := make([]SearchTerm, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, "AND") || strings.EqualFold(item, "OR") || strings.EqualFold(item, "NOT") {
			return nil, &UnsupportedFeatureError{Message: fmt.Sprintf("SOSL boolean search operator %s", strings.ToUpper(item))}
		}
		if strings.Contains(item, "?") {
			return nil, &UnsupportedFeatureError{Message: "SOSL fuzzy search operator ?"}
		}
		prefix := strings.HasSuffix(item, "*")
		item = strings.TrimSuffix(item, "*")
		if item != "" {
			terms = append(terms, SearchTerm{Text: item, Prefix: prefix})
		}
	}
	if len(terms) == 0 {
		return nil, p.errorf("empty FIND search term")
	}
	return terms, nil
}

func (p *parser) parseReturning(query *Query) error {
	for {
		if p.peek().kind != tokenWord {
			return p.errorf("expected RETURNING object")
		}
		object := p.next().text
		if !p.accept(tokenLParen) {
			return p.errorf("expected fields for %s", object)
		}
		returning, err := p.parseReturningObject(object)
		if err != nil {
			return err
		}
		query.Returning = append(query.Returning, returning)
		if !p.accept(tokenComma) {
			return nil
		}
		if p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, "WITH") {
			return p.errorf("unexpected comma before global SOSL clause")
		}
	}
}

func (p *parser) parseReturningObject(object string) (ReturningObject, error) {
	returning := ReturningObject{Object: object}
	for {
		switch {
		case p.accept(tokenRParen):
			return returning, nil
		case p.acceptKeyword("WHERE"):
			condition, err := p.parseCondition()
			if err != nil {
				return ReturningObject{}, err
			}
			returning.Where = &condition
		case p.acceptKeyword("ORDER"):
			if err := p.expectKeyword("BY"); err != nil {
				return ReturningObject{}, err
			}
			order, err := p.parseOrderBy()
			if err != nil {
				return ReturningObject{}, err
			}
			returning.OrderBy = order
		case p.acceptKeyword("LIMIT"):
			window, err := p.parseWindow("RETURNING LIMIT")
			if err != nil {
				return ReturningObject{}, err
			}
			returning.Limit = window
		case p.acceptKeyword("OFFSET"):
			window, err := p.parseWindow("RETURNING OFFSET")
			if err != nil {
				return ReturningObject{}, err
			}
			returning.Offset = window
		case p.accept(tokenComma):
			continue
		default:
			field, err := p.parseSelectExpr()
			if err != nil {
				return ReturningObject{}, err
			}
			returning.Fields = append(returning.Fields, field)
		}
	}
}

func (p *parser) parseSelectExpr() (SelectExpr, error) {
	field := p.next()
	if field.kind != tokenWord {
		return SelectExpr{}, p.errorf("expected RETURNING field")
	}
	if !p.accept(tokenLParen) {
		return SelectExpr{Field: field.text}, nil
	}
	if !isLocalFunction(field.text) {
		return SelectExpr{}, &UnsupportedFeatureError{Message: fmt.Sprintf("SOSL RETURNING %s function", field.text)}
	}
	argument := p.next()
	if argument.kind != tokenWord {
		return SelectExpr{}, p.errorf("expected %s field", field.text)
	}
	if !p.accept(tokenRParen) {
		return SelectExpr{}, p.errorf("expected end of %s expression", field.text)
	}
	alias := p.next()
	if alias.kind != tokenWord {
		return SelectExpr{}, p.errorf("expected alias for %s", field.text)
	}
	return SelectExpr{Field: argument.text, Func: strings.ToUpper(field.text), Alias: alias.text}, nil
}

func (p *parser) parseCondition() (Condition, error) {
	field := p.next()
	if field.kind != tokenWord {
		return Condition{}, p.errorf("expected SOSL WHERE field")
	}
	operator := p.next()
	var operation string
	switch {
	case operator.kind == tokenEqual:
		operation = "="
	case operator.kind == tokenNotEqual:
		operation = "!="
	case operator.kind == tokenWord && strings.EqualFold(operator.text, "LIKE"):
		operation = "LIKE"
	case operator.kind == tokenWord && strings.EqualFold(operator.text, "IN"):
		operation = "IN"
	default:
		return Condition{}, p.errorf("unsupported SOSL WHERE operator %q", operator.text)
	}
	if operation == "IN" {
		if p.accept(tokenColon) {
			bind := p.next()
			if bind.kind != tokenWord {
				return Condition{}, p.errorf("expected SOSL WHERE value bind")
			}
			return Condition{Field: field.text, Operator: operation, Bind: bind.text}, nil
		}
		if !p.accept(tokenLParen) {
			return Condition{}, p.errorf("expected SOSL WHERE IN values")
		}
		var values []string
		for !p.accept(tokenRParen) {
			value, bind, _, err := p.parseValue("SOSL WHERE IN value")
			if err != nil || bind != "" {
				return Condition{}, p.errorf("expected SOSL WHERE IN value")
			}
			values = append(values, value)
			if !p.accept(tokenComma) && p.peek().kind != tokenRParen {
				return Condition{}, p.errorf("expected SOSL WHERE IN comma or end")
			}
		}
		return Condition{Field: field.text, Operator: operation, Values: values}, nil
	}
	value, bind, nullValue, err := p.parseValue("SOSL WHERE value")
	if err != nil {
		return Condition{}, err
	}
	return Condition{Field: field.text, Operator: operation, Value: value, Bind: bind, ValueIsNull: nullValue}, nil
}

func (p *parser) parseOrderBy() ([]OrderSpec, error) {
	var order []OrderSpec
	for {
		field := p.next()
		if field.kind != tokenWord {
			return nil, p.errorf("expected SOSL ORDER BY field")
		}
		spec := OrderSpec{Field: field.text}
		if p.peek().kind == tokenWord && (strings.EqualFold(p.peek().text, "ASC") || strings.EqualFold(p.peek().text, "DESC")) {
			spec.Desc = strings.EqualFold(p.next().text, "DESC")
		}
		order = append(order, spec)
		if !p.accept(tokenComma) {
			return order, nil
		}
	}
}

func (p *parser) parseWindow(clause string) (Window, error) {
	if p.accept(tokenColon) {
		bind := p.next()
		if bind.kind != tokenWord {
			return Window{}, p.errorf("expected %s bind", clause)
		}
		return Window{Bind: bind.text}, nil
	}
	tok := p.next()
	if tok.kind != tokenNumber {
		return Window{}, p.errorf("expected %s value", clause)
	}
	value, err := strconv.Atoi(tok.text)
	if err != nil || value < 0 {
		return Window{}, p.errorf("invalid %s value %q", clause, tok.text)
	}
	return Window{Value: value, HasValue: true}, nil
}

func (p *parser) parseValue(clause string) (string, string, bool, error) {
	if p.accept(tokenColon) {
		bind := p.next()
		if bind.kind != tokenWord {
			return "", "", false, p.errorf("expected %s bind", clause)
		}
		return "", bind.text, false, nil
	}
	tok := p.next()
	if tok.kind != tokenString && tok.kind != tokenWord && tok.kind != tokenNumber {
		return "", "", false, p.errorf("expected %s", clause)
	}
	return tok.text, "", strings.EqualFold(tok.text, "NULL"), nil
}

func (p *parser) parseWith(query *Query) error {
	clause := p.next()
	if clause.kind != tokenWord {
		return p.errorf("expected SOSL WITH clause")
	}
	switch {
	case strings.EqualFold(clause.text, "SNIPPET"):
		query.WithSnippet = true
		return nil
	case strings.EqualFold(clause.text, "SPELL_CORRECTION"):
		if !p.accept(tokenEqual) {
			return p.errorf("expected SPELL_CORRECTION value")
		}
		value := p.next()
		if value.kind != tokenWord || (!strings.EqualFold(value.text, "TRUE") && !strings.EqualFold(value.text, "FALSE")) {
			return p.errorf("SOSL WITH SPELL_CORRECTION expects true or false")
		}
		parsed := strings.EqualFold(value.text, "TRUE")
		query.SpellCorrection = &parsed
		return nil
	case strings.EqualFold(clause.text, "DIVISION"):
		query.DivisionSpecified = true
		if !p.accept(tokenEqual) {
			return p.errorf("expected WITH DIVISION value")
		}
		value, bind, _, err := p.parseValue("WITH DIVISION")
		if err != nil {
			return err
		}
		query.DivisionBind = bind
		if bind == "" {
			query.DivisionBind = value
		}
		return nil
	case strings.EqualFold(clause.text, "PRICEBOOKID"):
		if !p.accept(tokenEqual) {
			return p.errorf("expected WITH PricebookId value")
		}
		value, bind, _, err := p.parseValue("WITH PricebookId")
		if err != nil {
			return err
		}
		query.PricebookID, query.PricebookIDBind = value, bind
		return nil
	case strings.EqualFold(clause.text, "DATA"):
		return &UnsupportedFeatureError{Message: "SOSL WITH DATA CATEGORY hosted search service"}
	case strings.EqualFold(clause.text, "DIVISIONFILTER"):
		return &UnsupportedFeatureError{Message: "SOSL WITH DivisionFilter hosted search service"}
	case strings.EqualFold(clause.text, "METADATA"):
		return &UnsupportedFeatureError{Message: "SOSL WITH METADATA hosted search service"}
	default:
		return &UnsupportedFeatureError{Message: fmt.Sprintf("SOSL WITH %s hosted search service", clause.text)}
	}
}

func isLocalFunction(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "FORMAT", "CONVERTCURRENCY", "TOLABEL":
		return true
	default:
		return false
	}
}

func (p *parser) expectKeyword(keyword string) error {
	tok := p.next()
	if tok.kind != tokenWord || !strings.EqualFold(tok.text, keyword) {
		return p.errorf("expected %s", keyword)
	}
	return nil
}

func (p *parser) acceptKeyword(keyword string) bool {
	if p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, keyword) {
		p.pos++
		return true
	}
	return false
}

func (p *parser) accept(kind tokenKind) bool {
	if p.peek().kind == kind {
		p.pos++
		return true
	}
	return false
}

func (p *parser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{kind: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) next() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("sosl: "+format, args...)
}

func lex(input string) []token {
	var tokens []token
	for i := 0; i < len(input); {
		if unicode.IsSpace(rune(input[i])) {
			i++
			continue
		}
		switch input[i] {
		case ',':
			tokens = append(tokens, token{kind: tokenComma, text: ","})
			i++
		case '(':
			tokens = append(tokens, token{kind: tokenLParen, text: "("})
			i++
		case ')':
			tokens = append(tokens, token{kind: tokenRParen, text: ")"})
			i++
		case '{':
			tokens = append(tokens, token{kind: tokenLBrace, text: "{"})
			i++
		case '}':
			tokens = append(tokens, token{kind: tokenRBrace, text: "}"})
			i++
		case ':':
			tokens = append(tokens, token{kind: tokenColon, text: ":"})
			i++
		case '=':
			tokens = append(tokens, token{kind: tokenEqual, text: "="})
			i++
		case '!':
			if i+1 < len(input) && input[i+1] == '=' {
				tokens = append(tokens, token{kind: tokenNotEqual, text: "!="})
				i += 2
			} else {
				tokens = append(tokens, token{kind: tokenInvalid, text: "!"})
				i++
			}
		case '\'', '"':
			quote := input[i]
			start := i + 1
			closed := false
			i++
			var value strings.Builder
			for i < len(input) {
				if input[i] == quote {
					if i+1 < len(input) && input[i+1] == quote {
						value.WriteString(input[start:i])
						value.WriteByte(quote)
						i += 2
						start = i
						continue
					}
					value.WriteString(input[start:i])
					i++
					tokens = append(tokens, token{kind: tokenString, text: value.String()})
					closed = true
					break
				}
				i++
			}
			if !closed {
				tokens = append(tokens, token{kind: tokenInvalid, text: input[start:]})
			}
		case '?':
			tokens = append(tokens, token{kind: tokenWord, text: "?"})
			i++
		default:
			if isDigit(input[i]) {
				start := i
				for i < len(input) && isDigit(input[i]) {
					i++
				}
				tokens = append(tokens, token{kind: tokenNumber, text: input[start:i]})
				continue
			}
			if isWordPart(input[i]) {
				start := i
				i++
				for i < len(input) && isWordPart(input[i]) {
					i++
				}
				tokens = append(tokens, token{kind: tokenWord, text: input[start:i]})
				continue
			}
			tokens = append(tokens, token{kind: tokenInvalid, text: string(input[i])})
			i++
		}
	}
	tokens = append(tokens, token{kind: tokenEOF})
	return tokens
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isWordPart(ch byte) bool {
	return ch == '_' || ch == '$' || ch == '.' || ch == '*' ||
		(ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || isDigit(ch)
}
