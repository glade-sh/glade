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
	program.Source = source
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
			if i < len(source) && source[i] == '.' && i+1 < len(source) && source[i+1] >= '0' && source[i+1] <= '9' {
				i++
				for i < len(source) && source[i] >= '0' && source[i] <= '9' {
					i++
				}
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
				case "==", "!=", "<=", ">=", "&&", "||", "++", "--", "+=", "-=":
					tokens = append(tokens, token{kind: tokenSymbol, text: two, pos: start})
					i += 2
					goto next
				}
			}
			switch source[i] {
			case '(', ')', '{', '}', '[', ']', ';', ',', '.', ':', '+', '-', '*', '/', '%', '=', '<', '>', '!', '|':
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
	if p.peek(tokenIdent, "System") && p.peekNext(tokenSymbol, ".") && p.peekN(2, tokenIdent, "runAs") {
		p.advance()
		p.advance()
		p.advance()
		if _, err := p.expect(tokenSymbol, "("); err != nil {
			return ir.Instruction{}, err
		}
		userExpr, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return ir.Instruction{}, err
		}
		body, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpRunAs, Expr: userExpr, Then: body, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "if") {
		if _, err := p.expect(tokenSymbol, "("); err != nil {
			return ir.Instruction{}, err
		}
		condition, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return ir.Instruction{}, err
		}
		thenBlock, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		var elseBlock []ir.Instruction
		if p.match(tokenIdent, "else") {
			elseBlock, err = p.parseStatementBlock()
			if err != nil {
				return ir.Instruction{}, err
			}
		}
		return ir.Instruction{Op: ir.OpIf, Expr: condition, Then: thenBlock, Else: elseBlock, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "while") {
		if _, err := p.expect(tokenSymbol, "("); err != nil {
			return ir.Instruction{}, err
		}
		condition, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return ir.Instruction{}, err
		}
		body, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpWhile, Expr: condition, Then: body, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "do") {
		body, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenIdent, "while"); err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, "("); err != nil {
			return ir.Instruction{}, err
		}
		condition, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpDoWhile, Expr: condition, Then: body, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "for") {
		return p.parseFor(start.pos)
	}

	if p.match(tokenIdent, "break") {
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpBreak, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "continue") {
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpContinue, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "throw") {
		if p.match(tokenSymbol, ";") {
			return ir.Instruction{Op: ir.OpThrow, Pos: start.pos}, nil
		}
		expr, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpThrow, Expr: expr, Pos: start.pos}, nil
	}

	if p.match(tokenIdent, "try") {
		tryBlock, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		inst := ir.Instruction{Op: ir.OpTry, Then: tryBlock, Pos: start.pos}
		for p.match(tokenIdent, "catch") {
			catchPos := p.tokens[p.pos-1].pos
			if _, err := p.expect(tokenSymbol, "("); err != nil {
				return ir.Instruction{}, err
			}
			catchType, err := p.parseTypeName()
			if err != nil {
				return ir.Instruction{}, err
			}
			catchTypes := []string{catchType}
			for p.match(tokenSymbol, "|") {
				nextType, err := p.parseTypeName()
				if err != nil {
					return ir.Instruction{}, err
				}
				catchTypes = append(catchTypes, nextType)
			}
			catchName, err := p.expect(tokenIdent, "")
			if err != nil {
				return ir.Instruction{}, err
			}
			if _, err := p.expect(tokenSymbol, ")"); err != nil {
				return ir.Instruction{}, err
			}
			catchBlock, err := p.parseStatementBlock()
			if err != nil {
				return ir.Instruction{}, err
			}
			inst.Catches = append(inst.Catches, ir.CatchClause{Types: catchTypes, Name: catchName.text, Body: catchBlock, Pos: catchPos})
			if len(inst.Catches) == 1 {
				inst.Type = catchType
				inst.CatchTypes = catchTypes
				inst.Name = catchName.text
				inst.Catch = catchBlock
			}
		}
		if p.match(tokenIdent, "finally") {
			finallyBlock, err := p.parseStatementBlock()
			if err != nil {
				return ir.Instruction{}, err
			}
			inst.Finally = finallyBlock
		}
		if len(inst.Catches) == 0 && len(inst.Catch) == 0 && len(inst.Finally) == 0 {
			return ir.Instruction{}, fmt.Errorf("try requires catch or finally at byte %d", start.pos)
		}
		return inst, nil
	}

	if p.match(tokenIdent, "switch") {
		return p.parseSwitch(start.pos)
	}

	for _, op := range []string{"insert", "update", "delete", "upsert", "undelete", "merge"} {
		if p.match(tokenIdent, op) {
			expr, err := p.parseExpression()
			if err != nil {
				return ir.Instruction{}, err
			}
			if op == "merge" {
				duplicate, err := p.parseExpression()
				if err != nil {
					return ir.Instruction{}, err
				}
				if _, err := p.expect(tokenSymbol, ";"); err != nil {
					return ir.Instruction{}, err
				}
				expr = ir.Expr{Kind: ir.ExprCall, Callee: "Database.merge", Args: []ir.Expr{
					expr,
					duplicate,
					{Kind: ir.ExprLiteral, Value: "true"},
				}}
				return ir.Instruction{Op: ir.OpDML, Name: op, Expr: expr, Pos: start.pos}, nil
			}
			field := ""
			if op == "upsert" && !p.peek(tokenSymbol, ";") {
				field, err = p.parseQualifiedName()
				if err != nil {
					return ir.Instruction{}, err
				}
			}
			if _, err := p.expect(tokenSymbol, ";"); err != nil {
				return ir.Instruction{}, err
			}
			return ir.Instruction{Op: ir.OpDML, Name: op, Expr: expr, Field: field, Pos: start.pos}, nil
		}
	}

	if p.match(tokenIdent, "return") {
		inst := ir.Instruction{Op: ir.OpReturn, Pos: start.pos}
		if !p.peek(tokenSymbol, ";") {
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

	if inst, ok, err := p.parseAssignmentLike(true); ok || err != nil {
		if err != nil {
			return ir.Instruction{}, err
		}
		return inst, nil
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

func (p *parser) parseFor(pos int) (ir.Instruction, error) {
	if _, err := p.expect(tokenSymbol, "("); err != nil {
		return ir.Instruction{}, err
	}
	save := p.pos
	if p.isDeclarationStart() {
		typeName, err := p.parseTypeName()
		if err != nil {
			return ir.Instruction{}, err
		}
		name, err := p.expect(tokenIdent, "")
		if err != nil {
			return ir.Instruction{}, err
		}
		if p.match(tokenSymbol, ":") {
			iterable, err := p.parseExpression()
			if err != nil {
				return ir.Instruction{}, err
			}
			if _, err := p.expect(tokenSymbol, ")"); err != nil {
				return ir.Instruction{}, err
			}
			body, err := p.parseStatementBlock()
			if err != nil {
				return ir.Instruction{}, err
			}
			return ir.Instruction{Op: ir.OpForEach, Type: typeName, Name: name.text, Expr: iterable, Then: body, Pos: pos}, nil
		}
		p.pos = save
	}

	var init *ir.Instruction
	if !p.peek(tokenSymbol, ";") {
		stmt, err := p.parseForPart()
		if err != nil {
			return ir.Instruction{}, err
		}
		init = &stmt
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	condition := ir.Expr{Kind: ir.ExprLiteral, Value: "true"}
	if !p.peek(tokenSymbol, ";") {
		expr, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		condition = expr
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	var update *ir.Instruction
	if !p.peek(tokenSymbol, ")") {
		stmt, err := p.parseForPart()
		if err != nil {
			return ir.Instruction{}, err
		}
		update = &stmt
	}
	if _, err := p.expect(tokenSymbol, ")"); err != nil {
		return ir.Instruction{}, err
	}
	body, err := p.parseStatementBlock()
	if err != nil {
		return ir.Instruction{}, err
	}
	return ir.Instruction{Op: ir.OpFor, Expr: condition, Init: init, Update: update, Then: body, Pos: pos}, nil
}

func (p *parser) parseForPart() (ir.Instruction, error) {
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
		return inst, nil
	}
	if inst, ok, err := p.parseAssignmentLike(false); ok || err != nil {
		return inst, err
	}
	expr, err := p.parseExpression()
	if err != nil {
		return ir.Instruction{}, err
	}
	return ir.Instruction{Op: ir.OpExpr, Expr: expr, Pos: start.pos}, nil
}

func (p *parser) parseAssignmentLike(requireSemicolon bool) (ir.Instruction, bool, error) {
	if !p.peek(tokenIdent, "") {
		return ir.Instruction{}, false, nil
	}
	save := p.pos
	start := p.tokens[p.pos]
	name, ok := p.parseAssignableName()
	if !ok {
		p.pos = save
		return ir.Instruction{}, false, nil
	}
	if p.match(tokenSymbol, "=") || p.match(tokenSymbol, "+=") || p.match(tokenSymbol, "-=") {
		op := p.tokens[p.pos-1].text
		expr, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, true, err
		}
		if op != "=" {
			operator := strings.TrimSuffix(op, "=")
			left := ir.Expr{Kind: ir.ExprVariable, Name: name}
			expr = binary(operator, left, expr)
		}
		if requireSemicolon {
			if _, err := p.expect(tokenSymbol, ";"); err != nil {
				return ir.Instruction{}, true, err
			}
		}
		return ir.Instruction{Op: ir.OpAssign, Name: name, Expr: expr, Pos: start.pos}, true, nil
	}
	if p.match(tokenSymbol, "++") || p.match(tokenSymbol, "--") {
		op := p.tokens[p.pos-1].text
		operator := "+"
		if op == "--" {
			operator = "-"
		}
		expr := binary(operator, ir.Expr{Kind: ir.ExprVariable, Name: name}, ir.Expr{Kind: ir.ExprLiteral, Value: "1"})
		if requireSemicolon {
			if _, err := p.expect(tokenSymbol, ";"); err != nil {
				return ir.Instruction{}, true, err
			}
		}
		return ir.Instruction{Op: ir.OpAssign, Name: name, Expr: expr, Pos: start.pos}, true, nil
	}
	p.pos = save
	return ir.Instruction{}, false, nil
}

func (p *parser) parseAssignableName() (string, bool) {
	if !p.peek(tokenIdent, "") {
		return "", false
	}
	name := p.advance().text
	for p.match(tokenSymbol, ".") {
		next, err := p.expect(tokenIdent, "")
		if err != nil {
			return "", false
		}
		name += "." + next.text
	}
	return name, true
}

func (p *parser) parseSwitch(pos int) (ir.Instruction, error) {
	if _, err := p.expect(tokenIdent, "on"); err != nil {
		return ir.Instruction{}, err
	}
	expr, err := p.parseExpression()
	if err != nil {
		return ir.Instruction{}, err
	}
	if _, err := p.expect(tokenSymbol, "{"); err != nil {
		return ir.Instruction{}, err
	}
	inst := ir.Instruction{Op: ir.OpSwitch, Expr: expr, Pos: pos}
	for !p.peek(tokenSymbol, "}") {
		caseStart := p.tokens[p.pos]
		if _, err := p.expect(tokenIdent, "when"); err != nil {
			return ir.Instruction{}, err
		}
		if p.match(tokenIdent, "else") {
			body, err := p.parseStatementBlock()
			if err != nil {
				return ir.Instruction{}, err
			}
			inst.Cases = append(inst.Cases, ir.SwitchCase{Else: true, Body: body, Pos: caseStart.pos})
			continue
		}
		var exprs []ir.Expr
		for {
			caseExpr, err := p.parseExpression()
			if err != nil {
				return ir.Instruction{}, err
			}
			exprs = append(exprs, caseExpr)
			if !p.match(tokenSymbol, ",") {
				break
			}
		}
		body, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		inst.Cases = append(inst.Cases, ir.SwitchCase{Exprs: exprs, Body: body, Pos: caseStart.pos})
	}
	if _, err := p.expect(tokenSymbol, "}"); err != nil {
		return ir.Instruction{}, err
	}
	return inst, nil
}

func (p *parser) parseStatementBlock() ([]ir.Instruction, error) {
	if !p.match(tokenSymbol, "{") {
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		return []ir.Instruction{stmt}, nil
	}
	var out []ir.Instruction
	for !p.peek(tokenSymbol, "}") {
		if p.peek(tokenEOF, "") {
			return nil, fmt.Errorf("unterminated block at byte %d", p.tokens[p.pos].pos)
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		out = append(out, stmt)
	}
	if _, err := p.expect(tokenSymbol, "}"); err != nil {
		return nil, err
	}
	return out, nil
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
		case p.match(tokenSymbol, "%"):
			right, err := p.parseUnary()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("%", left, right)
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
		if strings.Contains(tok.text, ".") {
			if _, err := strconv.ParseFloat(tok.text, 64); err != nil {
				return ir.Expr{}, fmt.Errorf("invalid decimal %q at byte %d", tok.text, tok.pos)
			}
		} else if _, err := strconv.ParseInt(tok.text, 10, 64); err != nil {
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
			args, namedArgs, err := p.parseNewArgs()
			if err != nil {
				return ir.Expr{}, err
			}
			return ir.Expr{Kind: ir.ExprCall, Callee: "new:" + typeName, Args: args, NamedArgs: namedArgs}, nil
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
		if tok.text == "[" {
			return p.parseSOQLLiteral(tok.pos)
		}
	}
	return ir.Expr{}, fmt.Errorf("unexpected token %q at byte %d", p.tokens[p.pos-1].text, p.tokens[p.pos-1].pos)
}

func (p *parser) parseNewArgs() ([]ir.Expr, []ir.NamedArg, error) {
	switch {
	case p.match(tokenSymbol, "("):
		if p.match(tokenSymbol, ")") {
			return nil, nil, nil
		}
		var args []ir.Expr
		var named []ir.NamedArg
		for {
			if p.peek(tokenIdent, "") && p.peekNext(tokenSymbol, "=") {
				name := p.advance().text
				p.advance()
				expr, err := p.parseExpression()
				if err != nil {
					return nil, nil, err
				}
				named = append(named, ir.NamedArg{Name: name, Expr: expr})
			} else {
				expr, err := p.parseExpression()
				if err != nil {
					return nil, nil, err
				}
				args = append(args, expr)
			}
			if p.match(tokenSymbol, ")") {
				return args, named, nil
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return nil, nil, err
			}
		}
	case p.match(tokenSymbol, "{"):
		if p.match(tokenSymbol, "}") {
			return nil, nil, nil
		}
		var args []ir.Expr
		for {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, nil, err
			}
			args = append(args, expr)
			if p.match(tokenSymbol, "}") {
				return args, nil, nil
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return nil, nil, err
			}
		}
	default:
		return nil, nil, fmt.Errorf("expected constructor arguments at byte %d", p.tokens[p.pos].pos)
	}
}

func (p *parser) parseSOQLLiteral(pos int) (ir.Expr, error) {
	depth := 1
	var parts []string
	for !p.peek(tokenEOF, "") {
		tok := p.advance()
		if tok.kind == tokenSymbol {
			switch tok.text {
			case "[":
				depth++
			case "]":
				depth--
				if depth == 0 {
					return ir.Expr{Kind: ir.ExprSOQL, Value: strings.Join(parts, " ")}, nil
				}
			}
		}
		if tok.kind == tokenString {
			parts = append(parts, "'"+tok.text+"'")
		} else {
			parts = append(parts, tok.text)
		}
	}
	return ir.Expr{}, fmt.Errorf("unterminated SOQL literal at byte %d", pos)
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
	return p.parseQualifiedName()
}

func (p *parser) parseQualifiedName() (string, error) {
	first, err := p.expect(tokenIdent, "")
	if err != nil {
		return "", err
	}
	name := first.text
	for p.match(tokenSymbol, ".") {
		next, err := p.expect(tokenIdent, "")
		if err != nil {
			return "", err
		}
		name += "." + next.text
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
	return p.peekN(1, kind, text)
}

func (p *parser) peekN(offset int, kind tokenKind, text string) bool {
	if p.pos+offset >= len(p.tokens) {
		return false
	}
	tok := p.tokens[p.pos+offset]
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
