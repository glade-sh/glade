package sosl

import (
	"fmt"
	"strings"
	"unicode"
)

type Query struct {
	Returning    []ReturningObject
	DivisionBind string
	LimitBind    string
}

type ReturningObject struct {
	Object     string
	Fields     []string
	WhereBinds []WhereBind
	LimitBind  string
	OffsetBind string
}

type WhereBind struct {
	Field string
	Name  string
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenComma
	tokenLParen
	tokenRParen
	tokenColon
	tokenEqual
	tokenNumber
)

type token struct {
	kind tokenKind
	text string
}

func Parse(input string) (Query, error) {
	tokens := lex(input)
	p := parser{tokens: tokens}
	if !p.skipToKeyword("RETURNING") {
		return Query{}, fmt.Errorf("sosl: missing RETURNING")
	}
	return p.parseReturning()
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) parseReturning() (Query, error) {
	var query Query
	for {
		p.skipCommas()
		if p.peek().kind == tokenEOF {
			break
		}
		if p.peek().kind == tokenIdent && equalFold(p.peek().text, "LIMIT") {
			p.next()
			bind, err := p.parseBindOrNumber("LIMIT")
			if err != nil {
				return Query{}, err
			}
			query.LimitBind = bind
			continue
		}
		if p.peek().kind == tokenIdent && equalFold(p.peek().text, "WITH") {
			p.next()
			if p.peek().kind == tokenIdent && equalFold(p.peek().text, "DIVISION") {
				p.next()
				if p.peek().kind == tokenEqual {
					p.next()
				}
				bind, err := p.parseDivisionValue()
				if err != nil {
					return Query{}, err
				}
				query.DivisionBind = bind
				continue
			}
			p.skipGlobalClause()
			continue
		}
		object := p.next()
		if object.kind != tokenIdent {
			return Query{}, fmt.Errorf("sosl: expected returning object")
		}
		if got := p.next(); got.kind != tokenLParen {
			return Query{}, fmt.Errorf("sosl: expected fields for %s", object.text)
		}
		fields, whereBinds, limitBind, offsetBind, err := p.parseFields()
		if err != nil {
			return Query{}, err
		}
		returning := ReturningObject{
			Object:     object.text,
			Fields:     fields,
			WhereBinds: whereBinds,
			LimitBind:  limitBind,
			OffsetBind: offsetBind,
		}
		query.Returning = append(query.Returning, returning)
	}
	if len(query.Returning) == 0 {
		return Query{}, fmt.Errorf("sosl: empty RETURNING clause")
	}
	return query, nil
}

func (p *parser) parseFields() ([]string, []WhereBind, string, string, error) {
	var fields []string
	for {
		tok := p.next()
		switch tok.kind {
		case tokenIdent:
			if equalFold(tok.text, "WHERE") {
				whereBinds, limitBind, offsetBind, err := p.parseWhereBinds()
				return fields, whereBinds, limitBind, offsetBind, err
			}
			if equalFold(tok.text, "ORDER") {
				p.skipReturningOrderBy()
				continue
			}
			if equalFold(tok.text, "LIMIT") {
				limitBind, offsetBind, err := p.parseReturningWindow("LIMIT")
				return fields, nil, limitBind, offsetBind, err
			}
			if equalFold(tok.text, "OFFSET") {
				limitBind, offsetBind, err := p.parseReturningWindow("OFFSET")
				return fields, nil, limitBind, offsetBind, err
			}
			fields = append(fields, tok.text)
		case tokenComma:
			continue
		case tokenRParen:
			return fields, nil, "", "", nil
		case tokenEOF:
			return nil, nil, "", "", fmt.Errorf("sosl: unterminated field list")
		default:
			return nil, nil, "", "", fmt.Errorf("sosl: expected field")
		}
	}
}

func (p *parser) skipReturningOrderBy() {
	if p.peek().kind == tokenIdent && equalFold(p.peek().text, "BY") {
		p.next()
	}
	for p.peek().kind != tokenEOF {
		if p.peek().kind == tokenRParen || (p.peek().kind == tokenIdent && (equalFold(p.peek().text, "LIMIT") || equalFold(p.peek().text, "OFFSET"))) {
			return
		}
		p.next()
	}
}

func (p *parser) parseWhereBinds() ([]WhereBind, string, string, error) {
	var binds []WhereBind
	depth := 0
	for {
		tok := p.next()
		switch tok.kind {
		case tokenEOF:
			return nil, "", "", fmt.Errorf("sosl: unterminated WHERE clause")
		case tokenLParen:
			depth++
		case tokenRParen:
			if depth == 0 {
				return binds, "", "", nil
			}
			depth--
		case tokenIdent:
			if depth == 0 && equalFold(tok.text, "LIMIT") {
				limitBind, offsetBind, err := p.parseReturningWindow("LIMIT")
				return binds, limitBind, offsetBind, err
			}
			if depth == 0 && equalFold(tok.text, "OFFSET") {
				limitBind, offsetBind, err := p.parseReturningWindow("OFFSET")
				return binds, limitBind, offsetBind, err
			}
			if p.peek().kind != tokenEqual {
				continue
			}
			p.next()
			if p.next().kind != tokenColon {
				continue
			}
			name := p.next()
			if name.kind == tokenIdent {
				binds = append(binds, WhereBind{Field: tok.text, Name: name.text})
			}
		}
	}
}

func (p *parser) parseReturningWindow(first string) (string, string, error) {
	var limitBind, offsetBind string
	if equalFold(first, "LIMIT") {
		bind, err := p.parseBindOrNumber("RETURNING LIMIT")
		if err != nil {
			return "", "", err
		}
		limitBind = bind
		if p.peek().kind == tokenIdent && equalFold(p.peek().text, "OFFSET") {
			p.next()
			bind, err := p.parseBindOrNumber("RETURNING OFFSET")
			if err != nil {
				return "", "", err
			}
			offsetBind = bind
		}
	} else {
		bind, err := p.parseBindOrNumber("RETURNING OFFSET")
		if err != nil {
			return "", "", err
		}
		offsetBind = bind
	}
	if p.next().kind != tokenRParen {
		return "", "", fmt.Errorf("sosl: expected end of fields after %s", first)
	}
	return limitBind, offsetBind, nil
}

func (p *parser) parseBindOrNumber(clause string) (string, error) {
	if p.peek().kind == tokenColon {
		p.next()
		bind := p.next()
		if bind.kind != tokenIdent {
			return "", fmt.Errorf("sosl: expected %s bind", clause)
		}
		return bind.text, nil
	}
	if p.next().kind != tokenNumber {
		return "", fmt.Errorf("sosl: expected %s value", clause)
	}
	return "", nil
}

func (p *parser) parseDivisionValue() (string, error) {
	if p.peek().kind == tokenColon {
		return p.parseBindOrNumber("WITH DIVISION")
	}
	if p.next().kind == tokenEOF {
		return "", fmt.Errorf("sosl: expected WITH DIVISION value")
	}
	return "", nil
}

func (p *parser) skipGlobalClause() {
	for p.peek().kind != tokenEOF {
		if p.peek().kind == tokenIdent && (equalFold(p.peek().text, "LIMIT") || equalFold(p.peek().text, "OFFSET")) {
			return
		}
		p.next()
	}
}

func (p *parser) skipToKeyword(keyword string) bool {
	for p.peek().kind != tokenEOF {
		tok := p.next()
		if tok.kind == tokenIdent && equalFold(tok.text, keyword) {
			return true
		}
	}
	return false
}

func (p *parser) skipCommas() {
	for p.peek().kind == tokenComma {
		p.pos++
	}
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

func lex(input string) []token {
	var tokens []token
	for i := 0; i < len(input); {
		r := rune(input[i])
		if unicode.IsSpace(r) {
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
		case ':':
			tokens = append(tokens, token{kind: tokenColon, text: ":"})
			i++
		case '=':
			tokens = append(tokens, token{kind: tokenEqual, text: "="})
			i++
		default:
			if '0' <= input[i] && input[i] <= '9' {
				start := i
				i++
				for i < len(input) && '0' <= input[i] && input[i] <= '9' {
					i++
				}
				tokens = append(tokens, token{kind: tokenNumber, text: input[start:i]})
				continue
			}
			if isIdentStart(input[i]) {
				start := i
				i++
				for i < len(input) && isIdentPart(input[i]) {
					i++
				}
				tokens = append(tokens, token{kind: tokenIdent, text: input[start:i]})
				continue
			}
			i++
		}
	}
	tokens = append(tokens, token{kind: tokenEOF})
	return tokens
}

func isIdentStart(ch byte) bool {
	return ch == '_' || ch == '$' || ('A' <= ch && ch <= 'Z') || ('a' <= ch && ch <= 'z')
}

func isIdentPart(ch byte) bool {
	return ch == '.' || isIdentStart(ch) || ('0' <= ch && ch <= '9')
}

func equalFold(a, b string) bool {
	return strings.EqualFold(a, b)
}
