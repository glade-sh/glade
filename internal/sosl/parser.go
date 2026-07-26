package sosl

import (
	"fmt"
	"strings"
	"unicode"
)

type Query struct {
	Returning []ReturningObject
	LimitBind string
}

type ReturningObject struct {
	Object string
	Fields []string
}

type tokenKind int

const (
	tokenEOF tokenKind = iota
	tokenIdent
	tokenComma
	tokenLParen
	tokenRParen
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
			bind := p.next()
			if bind.kind != tokenIdent {
				return Query{}, fmt.Errorf("sosl: expected LIMIT bind")
			}
			query.LimitBind = bind.text
			break
		}
		object := p.next()
		if object.kind != tokenIdent {
			return Query{}, fmt.Errorf("sosl: expected returning object")
		}
		if got := p.next(); got.kind != tokenLParen {
			return Query{}, fmt.Errorf("sosl: expected fields for %s", object.text)
		}
		fields, err := p.parseFields()
		if err != nil {
			return Query{}, err
		}
		query.Returning = append(query.Returning, ReturningObject{
			Object: object.text,
			Fields: fields,
		})
	}
	if len(query.Returning) == 0 {
		return Query{}, fmt.Errorf("sosl: empty RETURNING clause")
	}
	return query, nil
}

func (p *parser) parseFields() ([]string, error) {
	var fields []string
	for {
		tok := p.next()
		switch tok.kind {
		case tokenIdent:
			if equalFold(tok.text, "WHERE") {
				p.skipUntilObjectEnd()
				return fields, nil
			}
			fields = append(fields, tok.text)
		case tokenComma:
			continue
		case tokenRParen:
			return fields, nil
		case tokenEOF:
			return nil, fmt.Errorf("sosl: unterminated field list")
		default:
			return nil, fmt.Errorf("sosl: expected field")
		}
	}
}

func (p *parser) skipUntilObjectEnd() {
	depth := 0
	for {
		tok := p.next()
		switch tok.kind {
		case tokenEOF:
			return
		case tokenLParen:
			depth++
		case tokenRParen:
			if depth == 0 {
				return
			}
			depth--
		}
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
		default:
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
