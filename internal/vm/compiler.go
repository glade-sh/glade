package vm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/open-aer/oaer/internal/ir"
)

func CompileAnonymous(source string) (ir.Program, error) {
	tokens, err := lex(source)
	if err != nil {
		return ir.Program{}, err
	}
	p := parser{tokens: tokens}
	program, err := p.parseProgram()
	if err != nil {
		return ir.Program{}, err
	}
	return program, nil
}

type tokenKind string

const (
	tokenEOF    tokenKind = "eof"
	tokenIdent  tokenKind = "ident"
	tokenNumber tokenKind = "number"
	tokenString tokenKind = "string"
	tokenSymbol tokenKind = "symbol"
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func lex(source string) ([]token, error) {
	var tokens []token
	for i := 0; i < len(source); {
		r := rune(source[i])
		switch {
		case unicode.IsSpace(r):
			i++
		case isIdentStart(source[i]):
			start := i
			i++
			for i < len(source) && isIdentPart(source[i]) {
				i++
			}
			tokens = append(tokens, token{kind: tokenIdent, text: source[start:i], pos: start})
		case source[i] >= '0' && source[i] <= '9':
			start := i
			i++
			for i < len(source) && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			tokens = append(tokens, token{kind: tokenNumber, text: source[start:i], pos: start})
		case source[i] == '\'':
			start := i
			var text strings.Builder
			i++
			for i < len(source) {
				if source[i] == '\'' {
					if i+1 < len(source) && source[i+1] == '\'' {
						text.WriteByte('\'')
						i += 2
						continue
					}
					i++
					tokens = append(tokens, token{kind: tokenString, text: text.String(), pos: start})
					goto next
				}
				text.WriteByte(source[i])
				i++
			}
			return nil, fmt.Errorf("unterminated string literal at byte %d", start)
		default:
			start := i
			if i+1 < len(source) {
				two := source[i : i+2]
				switch two {
				case "==", "!=", "<=", ">=", "&&", "||":
					tokens = append(tokens, token{kind: tokenSymbol, text: two, pos: start})
					i += 2
					goto next
				}
			}
			switch source[i] {
			case '(', ')', '{', '}', '[', ']', ';', ',', '.', '+', '-', '*', '/', '=', '<', '>', '!':
				tokens = append(tokens, token{kind: tokenSymbol, text: source[i : i+1], pos: start})
				i++
			default:
				return nil, fmt.Errorf("unexpected character %q at byte %d", source[i], start)
			}
		}
	next:
	}
	tokens = append(tokens, token{kind: tokenEOF, pos: len(source)})
	return tokens, nil
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) parseProgram() (ir.Program, error) {
	var program ir.Program
	for !p.peek(tokenEOF, "") {
		stmt, err := p.parseStatement()
		if err != nil {
			return ir.Program{}, err
		}
		program.Instructions = append(program.Instructions, stmt)
	}
	return program, nil
}

func (p *parser) parseStatement() (ir.Instruction, error) {
	start := p.tokens[p.pos]
	if p.isDeclarationStart() {
		typeName, err := p.parseTypeName()
		if err != nil {
			return ir.Instruction{}, err
		}
		name, err := p.expect(tokenIdent, "")
		if err != nil {
			return ir.Instruction{}, err
		}
		inst := ir.Instruction{Op: ir.OpDeclare, Type: typeName, Name: name.text, Pos: start.pos}
		if p.match(tokenSymbol, "=") {
			expr, err := p.parseExpression()
			if err != nil {
				return ir.Instruction{}, err
			}
			inst.Expr = expr
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, err
		}
		return inst, nil
	}

	if p.peek(tokenIdent, "") && p.peekNext(tokenSymbol, "=") {
		name := p.advance().text
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpAssign, Name: name, Expr: expr, Pos: start.pos}, nil
	}

	expr, err := p.parseExpression()
	if err != nil {
		return ir.Instruction{}, err
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	return ir.Instruction{Op: ir.OpExpr, Expr: expr, Pos: start.pos}, nil
}

func (p *parser) parseExpression() (ir.Expr, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (ir.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return ir.Expr{}, err
	}
	for p.match(tokenSymbol, "||") {
		right, err := p.parseAnd()
		if err != nil {
			return ir.Expr{}, err
		}
		left = binary("||", left, right)
	}
	return left, nil
}

func (p *parser) parseAnd() (ir.Expr, error) {
	left, err := p.parseEquality()
	if err != nil {
		return ir.Expr{}, err
	}
	for p.match(tokenSymbol, "&&") {
		right, err := p.parseEquality()
		if err != nil {
			return ir.Expr{}, err
		}
		left = binary("&&", left, right)
	}
	return left, nil
}

func (p *parser) parseEquality() (ir.Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return ir.Expr{}, err
	}
	for {
		switch {
		case p.match(tokenSymbol, "=="):
			right, err := p.parseComparison()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("==", left, right)
		case p.match(tokenSymbol, "!="):
			right, err := p.parseComparison()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("!=", left, right)
		default:
			return left, nil
		}
	}
}

func (p *parser) parseComparison() (ir.Expr, error) {
	left, err := p.parseTerm()
	if err != nil {
		return ir.Expr{}, err
	}
	for {
		op := ""
		for _, candidate := range []string{"<=", ">=", "<", ">"} {
			if p.match(tokenSymbol, candidate) {
				op = candidate
				break
			}
		}
		if op == "" {
			return left, nil
		}
		right, err := p.parseTerm()
		if err != nil {
			return ir.Expr{}, err
		}
		left = binary(op, left, right)
	}
}

func (p *parser) parseTerm() (ir.Expr, error) {
	left, err := p.parseFactor()
	if err != nil {
		return ir.Expr{}, err
	}
	for {
		switch {
		case p.match(tokenSymbol, "+"):
			right, err := p.parseFactor()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("+", left, right)
		case p.match(tokenSymbol, "-"):
			right, err := p.parseFactor()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("-", left, right)
		default:
			return left, nil
		}
	}
}

func (p *parser) parseFactor() (ir.Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return ir.Expr{}, err
	}
	for {
		switch {
		case p.match(tokenSymbol, "*"):
			right, err := p.parseUnary()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("*", left, right)
		case p.match(tokenSymbol, "/"):
			right, err := p.parseUnary()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("/", left, right)
		default:
			return left, nil
		}
	}
}

func (p *parser) parseUnary() (ir.Expr, error) {
	switch {
	case p.match(tokenSymbol, "!"):
		expr, err := p.parseUnary()
		if err != nil {
			return ir.Expr{}, err
		}
		return ir.Expr{Kind: ir.ExprUnary, Operator: "!", Left: &expr}, nil
	case p.match(tokenSymbol, "-"):
		expr, err := p.parseUnary()
		if err != nil {
			return ir.Expr{}, err
		}
		return ir.Expr{Kind: ir.ExprUnary, Operator: "-", Left: &expr}, nil
	default:
		return p.parsePrimary()
	}
}

func (p *parser) parsePrimary() (ir.Expr, error) {
	switch tok := p.advance(); tok.kind {
	case tokenNumber:
		if _, err := strconv.ParseInt(tok.text, 10, 64); err != nil {
			return ir.Expr{}, fmt.Errorf("invalid integer %q at byte %d", tok.text, tok.pos)
		}
		return ir.Expr{Kind: ir.ExprLiteral, Value: tok.text}, nil
	case tokenString:
		return ir.Expr{Kind: ir.ExprLiteral, Value: "'" + strings.ReplaceAll(tok.text, "'", "''") + "'"}, nil
	case tokenIdent:
		if tok.text == "new" {
			typeName, err := p.parseTypeName()
			if err != nil {
				return ir.Expr{}, err
			}
			args, err := p.parseNewArgs()
			if err != nil {
				return ir.Expr{}, err
			}
			return ir.Expr{Kind: ir.ExprCall, Callee: "new:" + typeName, Args: args}, nil
		}
		switch tok.text {
		case "true", "false", "null":
			return ir.Expr{Kind: ir.ExprLiteral, Value: tok.text}, nil
		}
		name := tok.text
		for p.match(tokenSymbol, ".") {
			next, err := p.expect(tokenIdent, "")
			if err != nil {
				return ir.Expr{}, err
			}
			name += "." + next.text
		}
		if p.match(tokenSymbol, "(") {
			args, err := p.parseArguments()
			if err != nil {
				return ir.Expr{}, err
			}
			return ir.Expr{Kind: ir.ExprCall, Callee: name, Args: args}, nil
		}
		return ir.Expr{Kind: ir.ExprVariable, Name: name}, nil
	case tokenSymbol:
		if tok.text == "(" {
			expr, err := p.parseExpression()
			if err != nil {
				return ir.Expr{}, err
			}
			if _, err := p.expect(tokenSymbol, ")"); err != nil {
				return ir.Expr{}, err
			}
			return expr, nil
		}
	}
	return ir.Expr{}, fmt.Errorf("unexpected token %q at byte %d", p.tokens[p.pos-1].text, p.tokens[p.pos-1].pos)
}

func (p *parser) parseNewArgs() ([]ir.Expr, error) {
	switch {
	case p.match(tokenSymbol, "("):
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return nil, err
		}
		return nil, nil
	case p.match(tokenSymbol, "{"):
		if p.match(tokenSymbol, "}") {
			return nil, nil
		}
		var args []ir.Expr
		for {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			args = append(args, expr)
			if p.match(tokenSymbol, "}") {
				return args, nil
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("expected collection constructor at byte %d", p.tokens[p.pos].pos)
	}
}

func (p *parser) parseArguments() ([]ir.Expr, error) {
	if p.match(tokenSymbol, ")") {
		return nil, nil
	}
	var args []ir.Expr
	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		if p.match(tokenSymbol, ")") {
			return args, nil
		}
		if _, err := p.expect(tokenSymbol, ","); err != nil {
			return nil, err
		}
	}
}

func binary(op string, left, right ir.Expr) ir.Expr {
	return ir.Expr{Kind: ir.ExprBinary, Operator: op, Left: &left, Right: &right}
}

func (p *parser) isDeclarationStart() bool {
	if !p.peek(tokenIdent, "") {
		return false
	}
	if !isSupportedTypeStart(p.tokens[p.pos].text) {
		return false
	}
	save := p.pos
	if _, err := p.parseTypeName(); err != nil {
		p.pos = save
		return false
	}
	ok := p.peek(tokenIdent, "")
	p.pos = save
	return ok
}

func (p *parser) parseTypeName() (string, error) {
	first, err := p.expect(tokenIdent, "")
	if err != nil {
		return "", err
	}
	name := first.text
	if !isSupportedTypeStart(name) && !isSupportedScalarType(name) {
		return "", fmt.Errorf("unsupported type %q at byte %d", name, first.pos)
	}
	if p.match(tokenSymbol, "<") {
		var args []string
		for {
			arg, err := p.parseTypeName()
			if err != nil {
				return "", err
			}
			args = append(args, arg)
			if p.match(tokenSymbol, ">") {
				break
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return "", err
			}
		}
		name += "<" + strings.Join(args, ",") + ">"
	}
	if p.match(tokenSymbol, "[") {
		if _, err := p.expect(tokenSymbol, "]"); err != nil {
			return "", err
		}
		name += "[]"
	}
	return name, nil
}

func isSupportedTypeStart(name string) bool {
	switch name {
	case "Boolean", "Integer", "Long", "String", "Object", "List", "Set", "Map":
		return true
	default:
		return false
	}
}

func isSupportedScalarType(name string) bool {
	switch name {
	case "Boolean", "Integer", "Long", "String", "Object":
		return true
	default:
		return false
	}
}

func (p *parser) expect(kind tokenKind, text string) (token, error) {
	if p.peek(kind, text) {
		return p.advance(), nil
	}
	tok := p.tokens[p.pos]
	expected := string(kind)
	if text != "" {
		expected = text
	}
	return token{}, fmt.Errorf("expected %s at byte %d, got %q", expected, tok.pos, tok.text)
}

func (p *parser) match(kind tokenKind, text string) bool {
	if !p.peek(kind, text) {
		return false
	}
	p.pos++
	return true
}

func (p *parser) peek(kind tokenKind, text string) bool {
	tok := p.tokens[p.pos]
	if tok.kind != kind {
		return false
	}
	return text == "" || tok.text == text
}

func (p *parser) peekNext(kind tokenKind, text string) bool {
	if p.pos+1 >= len(p.tokens) {
		return false
	}
	tok := p.tokens[p.pos+1]
	if tok.kind != kind {
		return false
	}
	return text == "" || tok.text == text
}

func (p *parser) advance() token {
	tok := p.tokens[p.pos]
	p.pos++
	return tok
}

func isIdentStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func isIdentPart(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}
