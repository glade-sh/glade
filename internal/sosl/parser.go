package sosl

import (
	"errors"
	"fmt"
	"reflect"
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
	SearchScopeAll     SearchScope = "ALL FIELDS"
	SearchScopeName    SearchScope = "NAME FIELDS"
	SearchScopeEmail   SearchScope = "EMAIL FIELDS"
	SearchScopePhone   SearchScope = "PHONE FIELDS"
	SearchScopeSidebar SearchScope = "SIDEBAR FIELDS"
)

type SearchTerm struct {
	Text         string
	Prefix       bool
	NeedsMatcher bool
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
	ValueQuoted bool
	Bind        string
	Not         bool
	And         []Condition
	Or          []Condition
}

type OrderSpec struct {
	Field string
	Desc  bool
	Nulls string
}

type ReturningObject struct {
	Object  string
	Fields  []SelectExpr
	Where   *Condition
	OrderBy []OrderSpec
	Limit   Window
	Offset  Window
	// Inline negative literals are captured only with empty results (C011).
	// The VM rejects unproved nonempty behavior rather than inventing a window.
	EmptyNegativeOffset bool
}

type Query struct {
	Terms               []SearchTerm
	SearchText          string
	SearchBind          string
	MatchUnsupported    string
	Expression          *SearchExpression
	Scope               SearchScope
	Returning           []ReturningObject
	Limit               Window
	WithSnippet         bool
	SnippetTargetLength bool
	WithHighlight       bool
	UpdateViewStat      bool
	UpdateTracking      bool
	SpellCorrection     *bool
	PricebookID         string
	PricebookIDBind     string
	DivisionBind        string
	DivisionSpecified   bool
	AccessMode          string
	NetworkSpecified    bool
	NetworkNull         bool
	DataCategoryGroup   string
}

// ContractError distinguishes query syntax from runtime search validation.
type ContractError struct {
	Type, Message string
	InlineMessage string
}

func (err *ContractError) Error() string { return err.Message }

// InlineCompileDiagnosticMessage supplies the native compile rejection for
// captured inline grammar errors. Dynamic calls keep ContractError.Message.
func InlineCompileDiagnosticMessage(raw string, query Query, err error) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < len("FIND") || !strings.EqualFold(raw[:len("FIND")], "FIND") {
		return ""
	}
	if strings.HasPrefix(strings.TrimSpace(raw[len("FIND"):]), "{") {
		return "Unexpected token '{'."
	}
	var contract *ContractError
	if errors.As(err, &contract) {
		return contract.InlineMessage
	}
	if err == nil {
		for _, spec := range query.Returning {
			if spec.Limit.HasValue && spec.Limit.Value < 0 {
				return "Limit must be a non-negative value"
			}
		}
	}
	return ""
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
	tokenLess
	tokenLessEqual
	tokenGreater
	tokenGreaterEqual
	tokenInvalid
	tokenSearch
)

const escapedSearchWildcardMarker = "\uE000"

type token struct {
	kind  tokenKind
	text  string
	pos   int
	quote byte
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
	if p.peek().kind == tokenEOF {
		return Query{}, contract("QueryException", "Entities should be explictly specified in SOSL call in Apex")
	}
	if err := p.expectKeyword("FIND"); err != nil {
		return Query{}, err
	}
	terms, expression, text, bind, unsupported, err := p.parseTerms()
	if err != nil {
		return Query{}, err
	}
	query.Terms = terms
	query.Expression = expression
	query.SearchText, query.SearchBind, query.MatchUnsupported = text, bind, unsupported
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
		case p.acceptKeyword("SIDEBAR"):
			scope = SearchScopeSidebar
		default:
			word := p.next()
			if word.kind != tokenWord || strings.EqualFold(word.text, "FIELDS") || strings.EqualFold(word.text, "RETURNING") {
				return Query{}, contract("QueryException", "expecting a field or entity name, found '"+word.text+"'")
			}
			scope = SearchScope(strings.ToUpper(word.text) + " FIELDS")
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
		if query.DataCategoryGroup != "" && p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, "RETURNING") {
			return Query{}, contract("QueryException", "unexpected token: RETURNING")
		}
	}
	if err := p.expectKeyword("RETURNING"); err != nil {
		if p.peek().kind == tokenEOF {
			return Query{}, contract("QueryException", "Entities should be explictly specified in SOSL call in Apex")
		}
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
			for {
				kind := p.next()
				switch strings.ToUpper(kind.text) {
				case "VIEWSTAT":
					query.UpdateViewStat = true
				case "TRACKING":
					query.UpdateTracking = true
				default:
					return Query{}, contract("QueryException", "The update option is invalid.")
				}
				if !p.accept(tokenComma) {
					break
				}
			}
			if p.peek().kind != tokenEOF {
				return Query{}, &ContractError{Type: "QueryException", Message: "unexpected token: " + p.peek().text, InlineMessage: "Extra ']', at '" + p.peek().text + "'."}
			}
			continue
		}
		return Query{}, p.errorf("unexpected SOSL token %q", p.peek().text)
	}
	return query, nil
}

func (p *parser) parseTerms() ([]SearchTerm, *SearchExpression, string, string, string, error) {
	var text string
	switch p.peek().kind {
	case tokenSearch:
		tok := p.next()
		text = tok.text
		for i := 0; i < len(text); i++ {
			if text[i] == '\\' && i+1 < len(text) {
				i++
				continue
			}
			if strings.ContainsRune("&|+-", rune(text[i])) {
				return nil, nil, "", "", "", contract("QueryException", fmt.Sprintf("line 1:%d mismatched character '%c' expecting '}'", tok.pos+i, text[i]))
			}
		}
	case tokenString:
		if p.peek().quote == '"' {
			return nil, nil, "", "", "", contract("QueryException", fmt.Sprintf("line 1:%d no viable alternative at character '\"'", p.peek().pos))
		}
		text = p.next().text
	case tokenColon:
		p.next()
		bind := p.next()
		if bind.kind != tokenWord {
			return nil, nil, "", "", "", p.errorf("expected FIND bind")
		}
		return []SearchTerm{{Text: ":" + bind.text}}, nil, "", bind.text, "", nil
	default:
		found := p.next().text
		if found == "" {
			found = "<EOF>"
		}
		return nil, nil, "", "", "", &ContractError{Type: "QueryException", Message: "expecting a string literal, found '" + found + "'", InlineMessage: "Unexpected token '" + found + "'."}
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil, text, "", "", nil
	}
	terms, expression, err := parseSearchExpression(text)
	if err != nil {
		if strings.Contains(err.Error(), "empty SOSL search operand") || strings.Trim(text, "() \t\r\n") == "" {
			return nil, &SearchExpression{Operator: "INVALID_SEARCH_TERM"}, text, "", "", nil
		}
		// Salesforce accepts incomplete Boolean expressions. Do not invent
		// their index semantics; an actual candidate needs a supported matcher.
		return []SearchTerm{{Text: text}}, nil, text, "", "SOSL search expression matching", nil
	}
	return terms, expression, text, "", "", nil
}

func (p *parser) parseReturning(query *Query) error {
	for {
		if p.peek().kind != tokenWord {
			return p.errorf("expected RETURNING object")
		}
		object := p.next().text
		if !p.accept(tokenLParen) {
			query.Returning = append(query.Returning, ReturningObject{Object: object, Fields: []SelectExpr{{Field: "Id"}}})
			if p.accept(tokenComma) {
				continue
			}
			return nil
		}
		if p.peek().kind == tokenRParen {
			return contract("QueryException", "unexpected token: ')'")
		}
		returning, err := p.parseReturningObject(object)
		if err != nil {
			return err
		}
		duplicate := false
		for _, previous := range query.Returning {
			// C013 repeats the identical bucket. Distinct projections or filters
			// retain their existing behavior until captured independently.
			duplicate = duplicate || reflect.DeepEqual(previous, returning)
		}
		if !duplicate {
			query.Returning = append(query.Returning, returning)
		}
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
			if returning.Offset.HasValue || returning.Offset.Bind != "" {
				return ReturningObject{}, contract("QueryException", "expecting a right parentheses, found 'LIMIT'")
			}
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
	alias := argument.text
	if p.peek().kind == tokenWord && !isReturningClauseKeyword(p.peek().text) {
		alias = p.next().text
	} else if !strings.EqualFold(field.text, "toLabel") {
		return SelectExpr{}, p.errorf("expected alias for %s", field.text)
	}
	return SelectExpr{Field: argument.text, Func: strings.ToUpper(field.text), Alias: alias}, nil
}

func isReturningClauseKeyword(text string) bool {
	switch strings.ToUpper(strings.TrimSpace(text)) {
	case "WHERE", "ORDER", "LIMIT", "OFFSET", "WITH":
		return true
	default:
		return false
	}
}

func (p *parser) parseCondition() (Condition, error) {
	// Q008/Q030-Q033: NOT occupies its condition group. A surrounding
	// group is needed before joining it to another AND/OR operand.
	if p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, "NOT") {
		return p.parseConditionPrimary()
	}
	return p.parseConditionOr()
}

func (p *parser) parseConditionOr() (Condition, error) {
	first, err := p.parseConditionAnd()
	if err != nil {
		return Condition{}, err
	}
	conditions := []Condition{first}
	for p.acceptKeyword("OR") {
		if p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, "NOT") {
			return Condition{}, contract("QueryException", "unexpected token: '"+p.peek().text+"'")
		}
		condition, err := p.parseConditionAnd()
		if err != nil {
			return Condition{}, err
		}
		conditions = append(conditions, condition)
	}
	if len(conditions) == 1 {
		return first, nil
	}
	return Condition{Or: conditions}, nil
}

func (p *parser) parseConditionAnd() (Condition, error) {
	first, err := p.parseConditionPrimary()
	if err != nil {
		return Condition{}, err
	}
	conditions := []Condition{first}
	for p.acceptKeyword("AND") {
		condition, err := p.parseConditionPrimary()
		if err != nil {
			return Condition{}, err
		}
		conditions = append(conditions, condition)
	}
	if len(conditions) == 1 {
		return first, nil
	}
	return Condition{And: conditions}, nil
}

func (p *parser) parseConditionPrimary() (Condition, error) {
	if p.acceptKeyword("NOT") {
		condition, err := p.parseConditionPrimary()
		condition.Not = !condition.Not
		return condition, err
	}
	if p.accept(tokenLParen) {
		notGroup := p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, "NOT")
		condition, err := p.parseCondition()
		if err != nil {
			return Condition{}, err
		}
		if !p.accept(tokenRParen) {
			if notGroup {
				return Condition{}, contract("QueryException", "expecting a right parentheses, found '"+p.peek().text+"'")
			}
			return Condition{}, p.errorf("expected end of SOSL WHERE group")
		}
		return condition, nil
	}
	return p.parseConditionLeaf()
}

func (p *parser) parseConditionLeaf() (Condition, error) {
	field := p.next()
	if field.kind != tokenWord {
		return Condition{}, p.errorf("expected SOSL WHERE field")
	}
	operator := p.next()
	if operator.kind == tokenEqual && p.peek().kind == tokenEqual {
		text := operator.text
		for p.peek().kind == tokenEqual {
			text += p.next().text
		}
		return Condition{}, contract("QueryException", "unexpected token: '"+text+"'")
	}
	var operation string
	switch {
	case operator.kind == tokenEqual:
		operation = "="
	case operator.kind == tokenNotEqual:
		operation = "!="
	case operator.kind == tokenLess:
		operation = "<"
	case operator.kind == tokenLessEqual:
		operation = "<="
	case operator.kind == tokenGreater:
		operation = ">"
	case operator.kind == tokenGreaterEqual:
		operation = ">="
	case operator.kind == tokenWord && strings.EqualFold(operator.text, "LIKE"):
		operation = "LIKE"
	case operator.kind == tokenWord && strings.EqualFold(operator.text, "IN"):
		operation = "IN"
	case operator.kind == tokenWord && strings.EqualFold(operator.text, "NOT") && p.acceptKeyword("IN"):
		operation = "NOT IN"
	default:
		return Condition{}, p.errorf("unsupported SOSL WHERE operator %q", operator.text)
	}
	if operation == "IN" || operation == "NOT IN" {
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
	quoted := p.peek().kind == tokenString
	value, bind, nullValue, err := p.parseValue("SOSL WHERE value")
	if err != nil {
		return Condition{}, err
	}
	// Q026/Q037-Q040: a literal null is rejected for ordered comparisons.
	if nullValue && !quoted && (operation == "<" || operation == "<=" || operation == ">" || operation == ">=") {
		return Condition{}, contract("QueryException", "invalid operator")
	}
	return Condition{Field: field.text, Operator: operation, Value: value, Bind: bind, ValueIsNull: nullValue && !quoted, ValueQuoted: quoted}, nil
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
		if p.acceptKeyword("NULLS") {
			value := p.next()
			if !strings.EqualFold(value.text, "FIRST") && !strings.EqualFold(value.text, "LAST") {
				return nil, contract("QueryException", "unexpected token: '"+value.text+"'")
			}
			spec.Nulls = strings.ToUpper(value.text)
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
	if err != nil || (clause == "LIMIT" && value < 0) {
		found := tok.text
		if strings.HasPrefix(found, "-") {
			found = "-"
		}
		return Window{}, contract("QueryException", "expecting a colon, found '"+found+"'")
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
		if p.accept(tokenLParen) {
			query.SnippetTargetLength = true
			if err := p.expectKeyword("TARGET_LENGTH"); err != nil {
				return err
			}
			if !p.accept(tokenEqual) {
				return p.errorf("expected TARGET_LENGTH value")
			}
			value := p.next()
			if value.kind != tokenNumber || strings.HasPrefix(value.text, "-") {
				return contract("QueryException", "expecting a number, found '-'")
			}
			if !p.accept(tokenRParen) {
				return p.errorf("expected end of SNIPPET clause")
			}
		}
		return nil
	case strings.EqualFold(clause.text, "HIGHLIGHT"):
		query.WithHighlight = true
		return nil
	case strings.EqualFold(clause.text, "USER_MODE"), strings.EqualFold(clause.text, "SYSTEM_MODE"):
		query.AccessMode = strings.ToUpper(clause.text)
		return nil
	case strings.EqualFold(clause.text, "SPELL_CORRECTION"):
		if query.SpellCorrection != nil {
			return contract("QueryException", "Duplicate WITH SPELL_CORRECTION clause")
		}
		if !p.accept(tokenEqual) {
			return p.errorf("expected SPELL_CORRECTION value")
		}
		value := p.next()
		if value.kind == tokenWord && strings.EqualFold(value.text, "NULL") {
			return contract("QueryException", "Invalid WITH SPELL_CORRECTION clause")
		}
		if value.kind != tokenWord || (!strings.EqualFold(value.text, "TRUE") && !strings.EqualFold(value.text, "FALSE")) {
			return contract("QueryException", "unexpected token: '"+value.text+"'")
		}
		parsed := strings.EqualFold(value.text, "TRUE")
		query.SpellCorrection = &parsed
		return nil
	case strings.EqualFold(clause.text, "DIVISION"):
		query.DivisionSpecified = true
		if !p.accept(tokenEqual) {
			return p.errorf("expected WITH DIVISION value")
		}
		if p.peek().kind == tokenNumber {
			return contract("QueryException", "unexpected token: '"+p.next().text+"'")
		}
		if p.peek().kind == tokenWord && strings.EqualFold(p.peek().text, "NULL") {
			return contract("QueryException", "Invalid WITH DIVISION clause")
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
		if err := p.expectKeyword("CATEGORY"); err != nil {
			return err
		}
		query.DataCategoryGroup = p.next().text
		operator := p.next().text
		switch strings.ToUpper(operator) {
		case "AT", "ABOVE", "BELOW", "ABOVE_OR_BELOW":
		default:
			return p.errorf("invalid category operator %s", operator)
		}
		if p.next().kind != tokenWord {
			return p.errorf("expected category")
		}
		return nil
	case strings.EqualFold(clause.text, "DIVISIONFILTER"):
		return &UnsupportedFeatureError{Message: "SOSL WITH DivisionFilter hosted search service"}
	case strings.EqualFold(clause.text, "METADATA"):
		if !p.accept(tokenEqual) {
			return p.errorf("expected WITH METADATA value")
		}
		value := p.next()
		if value.kind == tokenWord {
			return contract("QueryException", "unexpected token: '"+value.text+"'")
		}
		return &UnsupportedFeatureError{Message: "SOSL WITH METADATA hosted search service"}
	case strings.EqualFold(clause.text, "NETWORK"):
		query.NetworkSpecified = true
		if p.acceptKeyword("IN") {
			if !p.accept(tokenLParen) {
				return p.errorf("expected NETWORK list")
			}
			for {
				if _, _, _, err := p.parseValue("NETWORK"); err != nil {
					return err
				}
				if !p.accept(tokenComma) {
					break
				}
			}
			if !p.accept(tokenRParen) {
				return p.errorf("expected end of NETWORK list")
			}
		} else {
			if !p.accept(tokenEqual) {
				return p.errorf("expected NETWORK value")
			}
			_, _, query.NetworkNull, _ = p.parseValue("NETWORK")
		}
		return nil
	default:
		return contract("QueryException", "Invalid WITH "+strings.ToUpper(clause.text)+" clause")
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
		return contract("QueryException", "unexpected token: "+tok.text)
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
			if len(tokens) == 1 && strings.EqualFold(tokens[0].text, "FIND") {
				start := i + 1
				i++
				for i < len(input) && input[i] != '}' {
					if input[i] == '\\' && i+1 < len(input) {
						i++
					}
					i++
				}
				if i < len(input) {
					tokens = append(tokens, token{kind: tokenSearch, text: input[start:i], pos: start})
					i++
				} else {
					tokens = append(tokens, token{kind: tokenInvalid, text: input[start:]})
				}
				continue
			}
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
		case '<':
			if i+1 < len(input) && input[i+1] == '=' {
				tokens = append(tokens, token{kind: tokenLessEqual, text: "<="})
				i += 2
			} else {
				tokens = append(tokens, token{kind: tokenLess, text: "<"})
				i++
			}
		case '>':
			if i+1 < len(input) && input[i+1] == '=' {
				tokens = append(tokens, token{kind: tokenGreaterEqual, text: ">="})
				i += 2
			} else {
				tokens = append(tokens, token{kind: tokenGreater, text: ">"})
				i++
			}
		case '\'', '"':
			quote := input[i]
			quotePos := i
			start := i + 1
			closed := false
			i++
			var value strings.Builder
			for i < len(input) {
				if input[i] == '\\' && i+1 < len(input) {
					if input[i+1] == '\\' {
						value.WriteString(input[start:i])
						value.WriteByte('\\')
						i += 2
						start = i
						continue
					}
					if input[i+1] == quote {
						value.WriteString(input[start:i])
						value.WriteByte(quote)
						i += 2
						start = i
						continue
					}
				}
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
					tokens = append(tokens, token{kind: tokenString, text: value.String(), quote: quote, pos: quotePos})
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
			if isDigit(input[i]) || ((input[i] == '-' || input[i] == '+') && i+1 < len(input) && isDigit(input[i+1])) {
				start := i
				if input[i] == '-' || input[i] == '+' {
					i++
				}
				for i < len(input) && isDigit(input[i]) {
					i++
				}
				if i+1 < len(input) && input[i] == '.' && isDigit(input[i+1]) {
					i++
					for i < len(input) && isDigit(input[i]) {
						i++
					}
				}
				tokens = append(tokens, token{kind: tokenNumber, text: input[start:i]})
				continue
			}
			if isWordPart(input[i]) || (input[i] == '\\' && i+1 < len(input) && input[i+1] == '*') {
				var word strings.Builder
				for i < len(input) {
					if input[i] == '\\' && i+1 < len(input) && input[i+1] == '*' {
						word.WriteString(`\*`)
						i += 2
						continue
					}
					if !isWordPart(input[i]) {
						break
					}
					word.WriteByte(input[i])
					i++
				}
				tokens = append(tokens, token{kind: tokenWord, text: word.String()})
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

func contract(kind, message string) error { return &ContractError{Type: kind, Message: message} }
