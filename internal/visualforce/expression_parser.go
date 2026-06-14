package visualforce

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/vm"
)

func (p *exprParser) parseExpression() (Expression, error) {
	return p.parseOr()
}

func (p *exprParser) parseOr() (Expression, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("||"), p.matchWord("OR"):
			right, err := p.parseAnd()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "||", left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseAnd() (Expression, error) {
	left, err := p.parseEquality()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("&&"), p.matchWord("AND"):
			right, err := p.parseEquality()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "&&", left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseEquality() (Expression, error) {
	left, err := p.parseCompare()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("=="):
			right, err := p.parseCompare()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "==", left: left, right: right}
		case p.matchOperator("!="):
			right, err := p.parseCompare()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "!=", left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseCompare() (Expression, error) {
	left, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator(">="):
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: ">=", left: left, right: right}
		case p.matchOperator("<="):
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "<=", left: left, right: right}
		case p.matchOperator(">"):
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: ">", left: left, right: right}
		case p.matchOperator("<"):
			right, err := p.parseAdd()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "<", left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseAdd() (Expression, error) {
	left, err := p.parseMultiply()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("+"):
			right, err := p.parseMultiply()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "+", left: left, right: right}
		case p.matchOperator("-"):
			right, err := p.parseMultiply()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "-", left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseMultiply() (Expression, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.matchOperator("*"):
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "*", left: left, right: right}
		case p.matchOperator("/"):
			right, err := p.parseUnary()
			if err != nil {
				return nil, err
			}
			left = binaryExpr{op: "/", left: left, right: right}
		default:
			return left, nil
		}
	}
}

func (p *exprParser) parseUnary() (Expression, error) {
	if p.matchOperator("!") {
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryExpr{op: "!", value: value}, nil
	}
	if p.matchWord("NOT") {
		value, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryExpr{op: "!", value: value}, nil
	}
	return p.parsePostfix()
}

func (p *exprParser) parsePostfix() (Expression, error) {
	expr, err := p.parseVisualforcePrimary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		if p.pos >= len(p.source) {
			return expr, nil
		}
		switch p.source[p.pos] {
		case '[':
			p.pos++
			key, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if p.pos >= len(p.source) || p.source[p.pos] != ']' {
				return nil, fmt.Errorf("missing ']' in index expression")
			}
			p.pos++
			expr = indexExpr{target: expr, key: key}
		case '.':
			p.pos++
			field := p.readIdent()
			if field == "" {
				return nil, fmt.Errorf("expected identifier after '.'")
			}
			p.skipSpace()
			if p.pos < len(p.source) && p.source[p.pos] == '(' {
				p.pos++
				args, err := p.parseArgList()
				if err != nil {
					return nil, err
				}
				p.skipSpace()
				if p.pos >= len(p.source) || p.source[p.pos] != ')' {
					return nil, fmt.Errorf("missing ')' in method expression")
				}
				p.pos++
				expr = methodCallExpr{target: expr, name: field, args: args}
				continue
			}
			expr = memberExpr{target: expr, field: field}
		default:
			return expr, nil
		}
	}
}

func (p *exprParser) parseVisualforcePrimary() (Expression, error) {
	p.skipSpace()
	if p.pos >= len(p.source) {
		return nil, errNoExpression
	}
	save := p.pos
	if p.matchWord("TRUE") {
		return literalExpr{value: vm.Bool(true)}, nil
	}
	p.pos = save
	if p.matchWord("FALSE") {
		return literalExpr{value: vm.Bool(false)}, nil
	}
	p.pos = save
	if p.matchWord("NULL") {
		return literalExpr{value: vm.Null}, nil
	}
	p.pos = save
	ch := p.source[p.pos]
	if isAlpha(ch) || ch == '_' || ch == '$' {
		return p.parseVisualforceIdentifierOrCall()
	}
	return p.parsePrimary()
}

func (p *exprParser) parseVisualforceIdentifierOrCall() (Expression, error) {
	parts, err := p.parseIdentifierParts()
	if err != nil {
		return nil, err
	}
	p.skipSpace()
	if p.pos < len(p.source) && p.source[p.pos] == '(' {
		p.pos++
		args, err := p.parseArgList()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if p.pos >= len(p.source) || p.source[p.pos] != ')' {
			return nil, fmt.Errorf("missing ')' in function expression")
		}
		p.pos++
		if len(parts) > 1 {
			return methodCallExpr{
				target: visualforceIdentifierExpr{parts: parts[:len(parts)-1]},
				name:   parts[len(parts)-1],
				args:   args,
			}, nil
		}
		return visualforceFunctionExpr{name: strings.ToLower(parts[0]), args: args}, nil
	}
	return visualforceIdentifierExpr{parts: parts}, nil
}

func (p *exprParser) parseNumber() (Expression, error) {
	start := p.pos
	for p.pos < len(p.source) && isDigit(p.source[p.pos]) {
		p.pos++
	}
	if p.pos < len(p.source) && p.source[p.pos] == '.' {
		p.pos++
		for p.pos < len(p.source) && isDigit(p.source[p.pos]) {
			p.pos++
		}
		parsed, err := strconv.ParseFloat(p.source[start:p.pos], 64)
		if err != nil {
			return nil, err
		}
		return literalExpr{value: vm.Decimal(parsed)}, nil
	}
	parsed, err := strconv.ParseInt(p.source[start:p.pos], 10, 64)
	if err != nil {
		return nil, err
	}
	return literalExpr{value: vm.Int(parsed)}, nil
}

func (p *exprParser) matchOperator(op string) bool {
	p.skipSpace()
	if p.pos+len(op) > len(p.source) || p.source[p.pos:p.pos+len(op)] != op {
		return false
	}
	p.pos += len(op)
	return true
}

func (p *exprParser) matchWord(word string) bool {
	p.skipSpace()
	if p.pos+len(word) > len(p.source) {
		return false
	}
	for i := range word {
		if !equalFoldASCII(p.source[p.pos+i], word[i]) {
			return false
		}
	}
	end := p.pos + len(word)
	if end < len(p.source) && isIdentChar(p.source[end]) {
		return false
	}
	p.pos = end
	return true
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func equalFoldASCII(left, right byte) bool {
	if left >= 'a' && left <= 'z' {
		left -= 'a' - 'A'
	}
	if right >= 'a' && right <= 'z' {
		right -= 'a' - 'A'
	}
	return left == right
}
