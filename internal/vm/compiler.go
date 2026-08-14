package vm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	apexparser "github.com/glade-sh/apex-parser"
	"github.com/glade-sh/glade/internal/ir"
)

type RuntimeLoweringError struct {
	Message string
}

func (e *RuntimeLoweringError) Error() string {
	return e.Message
}

type CompileOptions struct {
	APIVersion string
	Trigger    bool
}

func CompileAnonymous(source string) (ir.Program, error) {
	return CompileAnonymousWithOptions(source, CompileOptions{})
}

func CompileAnonymousWithOptions(source string, options CompileOptions) (ir.Program, error) {
	tokens, err := lex(source)
	if err != nil {
		return ir.Program{}, classifyCompileError(source, err)
	}
	p := parser{tokens: tokens}
	program, err := p.parseProgram()
	if err != nil {
		return ir.Program{}, classifyCompileError(source, err)
	}
	program.Source = source
	program.APIVersion = options.APIVersion
	program.Trigger = options.Trigger
	return program, nil
}

func classifyCompileError(source string, err error) error {
	if !runtimeLoweringSourceAccepted(source) {
		return err
	}
	return &RuntimeLoweringError{Message: err.Error()}
}

func runtimeLoweringSourceAccepted(source string) bool {
	parser := apexparser.NewParser()
	probes := []string{
		source,
		"public class GladeRuntimeLoweringProbe { public void run() {\n" + source + "\n} }",
	}
	for i, probe := range probes {
		if !parser.ParseSource(fmt.Sprintf("__glade_runtime_lowering_%d.cls", i), probe).HasErrors() {
			return true
		}
	}
	return false
}

func compileExpressionPrefix(source string) (ir.Expr, int, error) {
	tokens, err := lex(source)
	if err != nil {
		return ir.Expr{}, 0, err
	}
	p := parser{tokens: tokens}
	expr, err := p.parseExpression()
	if err != nil {
		return ir.Expr{}, 0, err
	}
	if p.pos >= len(p.tokens) || p.peek(tokenEOF, "") {
		return expr, len(source), nil
	}
	return expr, p.tokens[p.pos].pos, nil
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
		case i+1 < len(source) && source[i] == '/' && source[i+1] == '/':
			i += 2
			for i < len(source) && source[i] != '\n' && source[i] != '\r' {
				i++
			}
		case i+1 < len(source) && source[i] == '/' && source[i+1] == '*':
			start := i
			i += 2
			for i+1 < len(source) && !(source[i] == '*' && source[i+1] == '/') {
				i++
			}
			if i+1 >= len(source) {
				return nil, fmt.Errorf("unterminated block comment at byte %d", start)
			}
			i += 2
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
			if i < len(source) && (source[i] == 'e' || source[i] == 'E') {
				exp := i
				i++
				if i < len(source) && (source[i] == '+' || source[i] == '-') {
					i++
				}
				if i >= len(source) || source[i] < '0' || source[i] > '9' {
					i = exp
				} else {
					for i < len(source) && source[i] >= '0' && source[i] <= '9' {
						i++
					}
				}
			}
			if i < len(source) && strings.ContainsRune("LlDdFf", rune(source[i])) {
				i++
			}
			tokens = append(tokens, token{kind: tokenNumber, text: source[start:i], pos: start})
		case i+2 < len(source) && source[i:i+3] == "'''":
			var tok token
			var next int
			var err error
			_, singleNext, singleErr := lexSingleString(source, i)
			tripleOffset := strings.Index(source[i+3:], "'''")
			openingNewline := i+3 < len(source) && (source[i+3] == '\n' || source[i+3] == '\r')
			if tripleOffset >= 0 && (openingNewline || singleErr != nil || i+3+tripleOffset < singleNext) {
				tok, next, err = lexMultilineString(source, i)
			} else {
				tok, next, err = lexSingleString(source, i)
			}
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		case source[i] == '\'':
			tok, next, err := lexSingleString(source, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		default:
			start := i
			if i+2 < len(source) {
				three := source[i : i+3]
				switch three {
				case "===", "!==", "<<=", ">>=":
					tokens = append(tokens, token{kind: tokenSymbol, text: three, pos: start})
					i += 3
					goto next
				}
			}
			if i+1 < len(source) {
				two := source[i : i+2]
				switch two {
				case "==", "!=", "<>", "<=", ">=", "&&", "||", "++", "--", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "<<", "??":
					tokens = append(tokens, token{kind: tokenSymbol, text: two, pos: start})
					i += 2
					goto next
				case "?.":
					tokens = append(tokens, token{kind: tokenSymbol, text: "?.", pos: start})
					i += 2
					goto next
				}
			}
			switch source[i] {
			case '(', ')', '{', '}', '[', ']', ';', ',', '.', ':', '?', '+', '-', '*', '/', '%', '=', '<', '>', '!', '&', '|':
				tokens = append(tokens, token{kind: tokenSymbol, text: source[i : i+1], pos: start})
				i++
			default:
				if source[i] == '^' {
					return nil, fmt.Errorf("unexpected character %q at byte %d", source[i], start)
				}
				return nil, fmt.Errorf("unexpected character %q at byte %d", source[i], start)
			}
		}
	next:
	}
	tokens = append(tokens, token{kind: tokenEOF, pos: len(source)})
	return tokens, nil
}

func lexMultilineString(source string, start int) (token, int, error) {
	const delimiter = "'''"
	i := start + len(delimiter)
	var text strings.Builder
	if i >= len(source) || (source[i] != '\n' && source[i] != '\r') {
		return token{}, start, fmt.Errorf("multiline string opening delimiter must be followed by a newline at byte %d", start)
	}
	if i < len(source) && source[i] == '\r' {
		if i+1 < len(source) && source[i+1] == '\n' {
			i += 2
		}
	} else if i < len(source) && source[i] == '\n' {
		i++
	}
	for i < len(source) {
		if i+len(delimiter) <= len(source) && source[i:i+len(delimiter)] == delimiter {
			return token{kind: tokenString, text: text.String(), pos: start}, i + len(delimiter), nil
		}
		if source[i] == '\r' && i+1 < len(source) && source[i+1] == '\n' {
			text.WriteByte('\n')
			i += 2
			continue
		}
		text.WriteByte(source[i])
		i++
	}
	return token{}, start, fmt.Errorf("unterminated multiline string literal at byte %d", start)
}

func lexSingleString(source string, start int) (token, int, error) {
	i := start + 1
	var text strings.Builder
	for i < len(source) {
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				text.WriteByte('\'')
				i += 2
				continue
			}
			return token{kind: tokenString, text: text.String(), pos: start}, i + 1, nil
		}
		if source[i] == '\\' && i+1 < len(source) {
			switch source[i+1] {
			case '\'':
				if i+2 < len(source) && source[i+2] == '\'' && i+3 < len(source) && isIdentPart(source[i+3]) {
					text.WriteByte('\\')
					text.WriteByte('\'')
					i += 3
					continue
				}
				text.WriteByte('\'')
			case '\\':
				text.WriteByte('\\')
			case '"':
				text.WriteByte('"')
			case 'n':
				text.WriteByte('\n')
			case 'r':
				text.WriteByte('\r')
			case 't':
				text.WriteByte('\t')
			default:
				text.WriteByte('\\')
				text.WriteByte(source[i+1])
			}
			i += 2
			continue
		}
		text.WriteByte(source[i])
		i++
	}
	return token{}, start, fmt.Errorf("unterminated string literal at byte %d", start)
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
		runAsExpr := userExpr
		if p.match(tokenSymbol, ",") {
			packageExpr, err := p.parseExpression()
			if err != nil {
				return ir.Instruction{}, err
			}
			runAsExpr = ir.Expr{Kind: ir.ExprCall, Callee: "System.runAs", Args: []ir.Expr{userExpr, packageExpr}}
		}
		if _, err := p.expect(tokenSymbol, ")"); err != nil {
			return ir.Instruction{}, err
		}
		body, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpRunAs, Expr: runAsExpr, Then: body, Pos: start.pos}, nil
	}

	if p.peek(tokenSymbol, "{") {
		body, err := p.parseStatementBlock()
		if err != nil {
			return ir.Instruction{}, err
		}
		return ir.Instruction{Op: ir.OpBlock, Then: body, Pos: start.pos}, nil
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
		expr, err := p.parseAssignmentExpression()
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
			if err := validateLocalIdentifier(catchName); err != nil {
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
			mode, err := p.parseOptionalDMLAccessMode(op)
			if err != nil {
				return ir.Instruction{}, err
			}
			expr, err := p.parseExpression()
			if err != nil {
				return ir.Instruction{}, err
			}
			if op == "merge" {
				duplicate, err := p.parseExpression()
				if err != nil {
					return ir.Instruction{}, err
				}
				trailingMode, err := p.parseOptionalDMLAccessMode(op)
				if err != nil {
					return ir.Instruction{}, err
				}
				mode, err = mergeDMLModes(op, mode, trailingMode)
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
				return ir.Instruction{Op: ir.OpDML, Name: op, Expr: expr, DMLMode: mode, Pos: start.pos}, nil
			}
			field := ""
			if op == "upsert" && !p.peek(tokenSymbol, ";") {
				field, err = p.parseQualifiedName()
				if err != nil {
					return ir.Instruction{}, err
				}
			}
			trailingMode, err := p.parseOptionalDMLAccessMode(op)
			if err != nil {
				return ir.Instruction{}, err
			}
			mode, err = mergeDMLModes(op, mode, trailingMode)
			if err != nil {
				return ir.Instruction{}, err
			}
			if _, err := p.expect(tokenSymbol, ";"); err != nil {
				return ir.Instruction{}, err
			}
			return ir.Instruction{Op: ir.OpDML, Name: op, Expr: expr, Field: field, DMLMode: mode, Pos: start.pos}, nil
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

	if p.peek(tokenIdent, "final") && p.isDeclarationStartAfterFinal() {
		p.advance()
		return p.parseDeclaration(start.pos)
	}

	if p.isDeclarationStart() {
		return p.parseDeclaration(start.pos)
	}

	if inst, ok, err := p.parseComplexAssignmentLike(true); ok || err != nil {
		if err != nil {
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
	if inst, ok, err := p.parsePrefixIncrementLike(true); ok || err != nil {
		return inst, err
	}

	expr, err := p.parseAssignmentExpression()
	if err != nil {
		return ir.Instruction{}, err
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	return ir.Instruction{Op: ir.OpExpr, Expr: expr, Pos: start.pos}, nil
}

func (p *parser) parseOptionalDMLAccessMode(op string) (ir.DMLMode, error) {
	if !p.match(tokenIdent, "as") {
		return ir.DMLModeDefault, nil
	}
	if p.match(tokenIdent, "user") {
		return ir.DMLModeUser, nil
	}
	if p.match(tokenIdent, "system") {
		return ir.DMLModeSystem, nil
	}
	return ir.DMLModeDefault, fmt.Errorf("%s as expects user or system at byte %d", op, p.tokens[p.pos].pos)
}

func mergeDMLModes(op string, first, second ir.DMLMode) (ir.DMLMode, error) {
	if first != ir.DMLModeDefault && second != ir.DMLModeDefault {
		return ir.DMLModeDefault, fmt.Errorf("%s has duplicate access modes", op)
	}
	if first != ir.DMLModeDefault {
		return first, nil
	}
	return second, nil
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
		if err := validateLocalIdentifier(name); err != nil {
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
	var inits []ir.Instruction
	if !p.peek(tokenSymbol, ";") {
		stmts, err := p.parseForParts(";")
		if err != nil {
			return ir.Instruction{}, err
		}
		inits = stmts
		if len(stmts) > 0 {
			init = &stmts[0]
		}
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	condition := ir.Expr{Kind: ir.ExprLiteral, Value: "true"}
	if !p.peek(tokenSymbol, ";") {
		expr, err := p.parseAssignmentExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		condition = expr
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	var update *ir.Instruction
	var updates []ir.Instruction
	if !p.peek(tokenSymbol, ")") {
		stmts, err := p.parseForParts(")")
		if err != nil {
			return ir.Instruction{}, err
		}
		updates = stmts
		if len(stmts) > 0 {
			update = &stmts[0]
		}
	}
	if _, err := p.expect(tokenSymbol, ")"); err != nil {
		return ir.Instruction{}, err
	}
	body, err := p.parseStatementBlock()
	if err != nil {
		return ir.Instruction{}, err
	}
	return ir.Instruction{Op: ir.OpFor, Expr: condition, Init: init, Inits: inits, Update: update, Updates: updates, Then: body, Pos: pos}, nil
}

func (p *parser) parseForParts(end string) ([]ir.Instruction, error) {
	if p.peek(tokenIdent, "final") && p.isDeclarationStartAfterFinal() {
		p.advance()
		return p.parseForDeclarationParts()
	}
	if p.isDeclarationStart() {
		return p.parseForDeclarationParts()
	}
	var out []ir.Instruction
	for {
		stmt, err := p.parseForPart()
		if err != nil {
			return nil, err
		}
		out = append(out, stmt)
		if !p.match(tokenSymbol, ",") {
			return out, nil
		}
		if p.peek(tokenSymbol, end) {
			return nil, fmt.Errorf("expected for part after comma")
		}
	}
}

func (p *parser) parseForDeclarationParts() ([]ir.Instruction, error) {
	start := p.tokens[p.pos]
	typeName, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}
	var out []ir.Instruction
	for {
		name, err := p.expect(tokenIdent, "")
		if err != nil {
			return nil, err
		}
		if err := validateLocalIdentifier(name); err != nil {
			return nil, err
		}
		inst := ir.Instruction{Op: ir.OpDeclare, Type: typeName, Name: name.text, Pos: start.pos}
		if p.match(tokenSymbol, "=") {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			inst.Expr = expr
		}
		out = append(out, inst)
		if !p.match(tokenSymbol, ",") {
			return out, nil
		}
	}
}

func (p *parser) parseForPart() (ir.Instruction, error) {
	start := p.tokens[p.pos]
	if p.peek(tokenIdent, "final") && p.isDeclarationStartAfterFinal() {
		p.advance()
		return p.parseForDeclarationPart(start.pos)
	}
	if p.isDeclarationStart() {
		return p.parseForDeclarationPart(start.pos)
	}
	if inst, ok, err := p.parseAssignmentLike(false); ok || err != nil {
		return inst, err
	}
	if inst, ok, err := p.parsePrefixIncrementLike(false); ok || err != nil {
		return inst, err
	}
	expr, err := p.parseExpression()
	if err != nil {
		return ir.Instruction{}, err
	}
	return ir.Instruction{Op: ir.OpExpr, Expr: expr, Pos: start.pos}, nil
}

func (p *parser) parseDeclaration(pos int) (ir.Instruction, error) {
	insts, err := p.parseForDeclarationParts()
	if err != nil {
		return ir.Instruction{}, err
	}
	if _, err := p.expect(tokenSymbol, ";"); err != nil {
		return ir.Instruction{}, err
	}
	if len(insts) == 1 {
		return insts[0], nil
	}
	// Multi-declarator statements share the enclosing scope; do not use OpBlock.
	return ir.Instruction{Op: ir.OpDeclGroup, Then: insts, Pos: pos}, nil
}

func (p *parser) parseForDeclarationPart(pos int) (ir.Instruction, error) {
	typeName, err := p.parseTypeName()
	if err != nil {
		return ir.Instruction{}, err
	}
	name, err := p.expect(tokenIdent, "")
	if err != nil {
		return ir.Instruction{}, err
	}
	if err := validateLocalIdentifier(name); err != nil {
		return ir.Instruction{}, err
	}
	inst := ir.Instruction{Op: ir.OpDeclare, Type: typeName, Name: name.text, Pos: pos}
	if p.match(tokenSymbol, "=") {
		expr, err := p.parseExpression()
		if err != nil {
			return ir.Instruction{}, err
		}
		inst.Expr = expr
	}
	return inst, nil
}

func validateLocalIdentifier(name token) error {
	if apexparser.IsReservedSourceIdentifier(name.text, false) {
		return fmt.Errorf("identifier name is reserved: %s at byte %d", name.text, name.pos)
	}
	if err := apexparser.ValidateSourceIdentifier(name.text); err != nil {
		return fmt.Errorf("%v at byte %d", err, name.pos)
	}
	return nil
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
	if p.match(tokenSymbol, "=") || p.match(tokenSymbol, "+=") || p.match(tokenSymbol, "-=") || p.match(tokenSymbol, "*=") || p.match(tokenSymbol, "/=") || p.match(tokenSymbol, "%=") || p.match(tokenSymbol, "&=") || p.match(tokenSymbol, "|=") || p.match(tokenSymbol, "<<=") || p.match(tokenSymbol, ">>=") {
		op := p.tokens[p.pos-1].text
		expr, err := p.parseAssignmentExpression()
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

func (p *parser) parseComplexAssignmentLike(requireSemicolon bool) (ir.Instruction, bool, error) {
	save := p.pos
	start := p.tokens[p.pos]
	left, err := p.parseTernary()
	if err != nil {
		p.pos = save
		return ir.Instruction{}, false, nil
	}
	if !p.match(tokenSymbol, "=") && !p.match(tokenSymbol, "+=") && !p.match(tokenSymbol, "-=") && !p.match(tokenSymbol, "*=") && !p.match(tokenSymbol, "/=") && !p.match(tokenSymbol, "%=") && !p.match(tokenSymbol, "&=") && !p.match(tokenSymbol, "|=") && !p.match(tokenSymbol, "<<=") && !p.match(tokenSymbol, ">>=") {
		p.pos = save
		return ir.Instruction{}, false, nil
	}
	op := p.tokens[p.pos-1].text
	if left.Kind != ir.ExprCall || left.Left == nil || (left.Callee != "get" && !strings.HasPrefix(left.Callee, "__field:")) {
		p.pos = save
		return ir.Instruction{}, false, nil
	}
	value, err := p.parseAssignmentExpression()
	if err != nil {
		return ir.Instruction{}, true, err
	}
	if op != "=" {
		value = binary(strings.TrimSuffix(op, "="), left, value)
	}
	if requireSemicolon {
		if _, err := p.expect(tokenSymbol, ";"); err != nil {
			return ir.Instruction{}, true, err
		}
	}
	if left.Kind == ir.ExprCall && left.Callee == "get" && left.Left != nil && len(left.Args) == 1 {
		receiver := *left.Left
		return ir.Instruction{Op: ir.OpExpr, Expr: ir.Expr{Kind: ir.ExprCall, Callee: "set", Left: &receiver, Args: []ir.Expr{left.Args[0], value}}, Pos: start.pos}, true, nil
	}
	if left.Kind != ir.ExprCall || !strings.HasPrefix(left.Callee, "__field:") || left.Left == nil {
		p.pos = save
		return ir.Instruction{}, false, nil
	}
	receiver := *left.Left
	field := strings.TrimPrefix(left.Callee, "__field:")
	return ir.Instruction{Op: ir.OpExpr, Expr: ir.Expr{Kind: ir.ExprCall, Callee: "__assignField:" + field, Left: &receiver, Args: []ir.Expr{value}}, Pos: start.pos}, true, nil
}

func (p *parser) parsePrefixIncrementLike(requireSemicolon bool) (ir.Instruction, bool, error) {
	if !p.peek(tokenSymbol, "++") && !p.peek(tokenSymbol, "--") {
		return ir.Instruction{}, false, nil
	}
	save := p.pos
	start := p.advance()
	name, ok := p.parseAssignableName()
	if !ok {
		p.pos = save
		return ir.Instruction{}, false, nil
	}
	operator := "+"
	if start.text == "--" {
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

func (p *parser) parseAssignmentExpression() (ir.Expr, error) {
	save := p.pos
	name, ok := p.parseAssignableName()
	if ok && p.peekAnySymbol("=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "<<=", ">>=") && !p.peekNext(tokenSymbol, ">") {
		op := p.advance().text
		value, err := p.parseAssignmentExpression()
		if err != nil {
			return ir.Expr{}, err
		}
		if op != "=" {
			value = binary(strings.TrimSuffix(op, "="), ir.Expr{Kind: ir.ExprVariable, Name: name}, value)
		}
		return ir.Expr{Kind: ir.ExprCall, Callee: "__assign:" + name, Args: []ir.Expr{value}}, nil
	}
	p.pos = save
	left, err := p.parseTernary()
	if err != nil {
		return ir.Expr{}, err
	}
	if !p.peek(tokenSymbol, "=") || p.peekNext(tokenSymbol, ">") {
		return left, nil
	}
	if left.Kind != ir.ExprCall || left.Left == nil || (left.Callee != "get" && !strings.HasPrefix(left.Callee, "__field:")) {
		return left, nil
	}
	p.advance()
	value, err := p.parseAssignmentExpression()
	if err != nil {
		return ir.Expr{}, err
	}
	if left.Callee == "get" && len(left.Args) == 1 {
		receiver := *left.Left
		return ir.Expr{Kind: ir.ExprCall, Callee: "set", Left: &receiver, Args: []ir.Expr{left.Args[0], value}}, nil
	}
	field := strings.TrimPrefix(left.Callee, "__field:")
	receiver := *left.Left
	return ir.Expr{Kind: ir.ExprCall, Callee: "__assignField:" + field, Left: &receiver, Args: []ir.Expr{value}}, nil
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
			if caseExpr.Kind == ir.ExprVariable && p.peek(tokenIdent, "") {
				binding := p.advance()
				if err := validateLocalIdentifier(binding); err != nil {
					return ir.Instruction{}, err
				}
				caseExpr.Name = "__typecase:" + caseExpr.Name + ":" + binding.text
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
	return p.parseAssignmentExpression()
}

func (p *parser) parseTernary() (ir.Expr, error) {
	condition, err := p.parseNullCoalesce()
	if err != nil {
		return ir.Expr{}, err
	}
	if !p.match(tokenSymbol, "?") {
		return condition, nil
	}
	whenTrue, err := p.parseExpression()
	if err != nil {
		return ir.Expr{}, err
	}
	if _, err := p.expect(tokenSymbol, ":"); err != nil {
		return ir.Expr{}, err
	}
	whenFalse, err := p.parseExpression()
	if err != nil {
		return ir.Expr{}, err
	}
	return ir.Expr{Kind: ir.ExprCall, Callee: "__ternary", Args: []ir.Expr{condition, whenTrue, whenFalse}}, nil
}

func (p *parser) parseNullCoalesce() (ir.Expr, error) {
	left, err := p.parseOr()
	if err != nil {
		return ir.Expr{}, err
	}
	for p.match(tokenSymbol, "??") {
		right, err := p.parseOr()
		if err != nil {
			return ir.Expr{}, err
		}
		left = ir.Expr{Kind: ir.ExprCall, Callee: "__coalesce", Args: []ir.Expr{left, right}}
	}
	return left, nil
}

func (p *parser) parseOr() (ir.Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return ir.Expr{}, err
	}
	for p.matchAnySymbol("||", "|") {
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
	for p.matchAnySymbol("&&", "&") {
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
		case p.match(tokenSymbol, "==="):
			right, err := p.parseComparison()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("===", left, right)
		case p.matchAnySymbol("!=", "<>"):
			right, err := p.parseComparison()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("!=", left, right)
		case p.match(tokenSymbol, "!=="):
			right, err := p.parseComparison()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("!==", left, right)
		default:
			return left, nil
		}
	}
}

func (p *parser) parseComparison() (ir.Expr, error) {
	left, err := p.parseShift()
	if err != nil {
		return ir.Expr{}, err
	}
	for {
		if p.match(tokenIdent, "instanceof") {
			typeName, err := p.parseTypeName()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("instanceof", left, ir.Expr{Kind: ir.ExprVariable, Name: typeName})
			continue
		}
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
		right, err := p.parseShift()
		if err != nil {
			return ir.Expr{}, err
		}
		left = binary(op, left, right)
	}
}

func (p *parser) parseShift() (ir.Expr, error) {
	left, err := p.parseTerm()
	if err != nil {
		return ir.Expr{}, err
	}
	for {
		switch {
		case p.match(tokenSymbol, "<<"):
			right, err := p.parseTerm()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary("<<", left, right)
		case p.match(tokenSymbol, ">>"):
			right, err := p.parseTerm()
			if err != nil {
				return ir.Expr{}, err
			}
			left = binary(">>", left, right)
		default:
			return left, nil
		}
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
	case p.match(tokenSymbol, "++") || p.match(tokenSymbol, "--"):
		op := p.tokens[p.pos-1].text
		expr, err := p.parseUnary()
		if err != nil {
			return ir.Expr{}, err
		}
		return ir.Expr{Kind: ir.ExprCall, Callee: "__prefix:" + op, Left: &expr}, nil
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
	case p.match(tokenSymbol, "+"):
		expr, err := p.parseUnary()
		if err != nil {
			return ir.Expr{}, err
		}
		return ir.Expr{Kind: ir.ExprUnary, Operator: "+", Left: &expr}, nil
	default:
		expr, err := p.parsePrimary()
		if err != nil {
			return ir.Expr{}, err
		}
		return p.parsePostfix(expr)
	}
}

func (p *parser) parsePostfix(expr ir.Expr) (ir.Expr, error) {
	for {
		if p.match(tokenSymbol, "[") {
			index, err := p.parseExpression()
			if err != nil {
				return ir.Expr{}, err
			}
			if _, err := p.expect(tokenSymbol, "]"); err != nil {
				return ir.Expr{}, err
			}
			receiver := expr
			expr = ir.Expr{Kind: ir.ExprCall, Callee: "get", Args: []ir.Expr{index}, Left: &receiver}
			continue
		}
		if p.matchAnySymbol(".", "?.") {
			safe := p.tokens[p.pos-1].text == "?."
			member, err := p.expect(tokenIdent, "")
			if err != nil {
				return ir.Expr{}, err
			}
			receiver := expr
			if !p.match(tokenSymbol, "(") {
				prefix := "__field:"
				if safe {
					prefix = "__safe_field:"
				}
				expr = ir.Expr{Kind: ir.ExprCall, Callee: prefix + member.text, Left: &receiver}
				continue
			}
			args, err := p.parseArguments()
			if err != nil {
				return ir.Expr{}, err
			}
			callee := member.text
			if safe {
				callee = "__safe_call:" + callee
			}
			expr = ir.Expr{Kind: ir.ExprCall, Callee: callee, Args: args, Left: &receiver}
			continue
		}
		if p.match(tokenSymbol, "++") || p.match(tokenSymbol, "--") {
			op := p.tokens[p.pos-1].text
			left := expr
			expr = ir.Expr{Kind: ir.ExprCall, Callee: "__postfix:" + op, Left: &left}
			continue
		}
		break
	}
	return expr, nil
}

func (p *parser) parsePrimary() (ir.Expr, error) {
	switch tok := p.advance(); tok.kind {
	case tokenNumber:
		numberText, isDecimal := parseNumberTokenText(tok.text)
		if isDecimal {
			if _, err := strconv.ParseFloat(numberText, 64); err != nil {
				return ir.Expr{}, fmt.Errorf("invalid decimal %q at byte %d", tok.text, tok.pos)
			}
		} else if _, err := strconv.ParseInt(numberText, 10, 64); err != nil {
			return ir.Expr{}, fmt.Errorf("invalid integer %q at byte %d", tok.text, tok.pos)
		}
		return ir.Expr{Kind: ir.ExprLiteral, Value: tok.text}, nil
	case tokenString:
		return ir.Expr{Kind: ir.ExprLiteral, Value: apexStringLiteralFromTokenText(tok.text)}, nil
	case tokenIdent:
		if strings.EqualFold(tok.text, "new") {
			typeName, size, err := p.parseNewTypeName()
			if err != nil {
				return ir.Expr{}, err
			}
			if size != nil {
				return ir.Expr{Kind: ir.ExprCall, Callee: "__newArray:" + typeName, Args: []ir.Expr{*size}}, nil
			}
			args, namedArgs, literalArgs, err := p.parseNewArgs()
			if err != nil {
				return ir.Expr{}, err
			}
			callee := "new:" + typeName
			if literalArgs {
				callee = "newlit:" + typeName
			}
			return ir.Expr{Kind: ir.ExprCall, Callee: callee, Args: args, NamedArgs: namedArgs}, nil
		}
		switch strings.ToLower(tok.text) {
		case "true", "false", "null":
			return ir.Expr{Kind: ir.ExprLiteral, Value: strings.ToLower(tok.text)}, nil
		}
		name := tok.text
		for p.match(tokenSymbol, ".") {
			next, err := p.expect(tokenIdent, "")
			if err != nil {
				return ir.Expr{}, err
			}
			name += "." + next.text
		}
		if classLiteral, ok, err := p.parseClassLiteralSuffix(name); ok || err != nil {
			return classLiteral, err
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
			if expr, ok, err := p.parseCastExpression(); ok || err != nil {
				return expr, err
			}
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
		if tok.text == "." && p.peek(tokenNumber, "") {
			number := p.advance()
			text := "." + number.text
			if parsed, isDecimal := parseNumberTokenText(text); !isDecimal {
				return ir.Expr{}, fmt.Errorf("invalid decimal %q at byte %d", text, tok.pos)
			} else if _, err := strconv.ParseFloat(parsed, 64); err != nil {
				return ir.Expr{}, fmt.Errorf("invalid decimal %q at byte %d", text, tok.pos)
			}
			return ir.Expr{Kind: ir.ExprLiteral, Value: text}, nil
		}
	}
	return ir.Expr{}, fmt.Errorf("unexpected token %q at byte %d", p.tokens[p.pos-1].text, p.tokens[p.pos-1].pos)
}

func apexStringLiteralFromTokenText(text string) string {
	var out strings.Builder
	out.Grow(len(text) + 2)
	out.WriteByte('\'')
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			out.WriteString("''")
		case '\\':
			out.WriteString("\\\\")
		case '\n':
			out.WriteString("\\n")
		case '\r':
			out.WriteString("\\r")
		case '\t':
			out.WriteString("\\t")
		default:
			out.WriteByte(text[i])
		}
	}
	out.WriteByte('\'')
	return out.String()
}

func (p *parser) parseClassLiteralSuffix(name string) (ir.Expr, bool, error) {
	save := p.pos
	typeName, err := p.parseTypeSuffix(name)
	if err != nil {
		p.pos = save
		return ir.Expr{}, false, nil
	}
	if !p.match(tokenSymbol, ".") {
		p.pos = save
		return ir.Expr{}, false, nil
	}
	classToken, err := p.expect(tokenIdent, "class")
	if err != nil {
		p.pos = save
		return ir.Expr{}, false, nil
	}
	return ir.Expr{Kind: ir.ExprVariable, Name: typeName + "." + classToken.text}, true, nil
}

func (p *parser) parseTypeSuffix(name string) (string, error) {
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

func (p *parser) parseNewTypeName() (string, *ir.Expr, error) {
	first, err := p.expect(tokenIdent, "")
	if err != nil {
		return "", nil, err
	}
	name := first.text
	for p.match(tokenSymbol, ".") {
		next, err := p.expect(tokenIdent, "")
		if err != nil {
			return "", nil, err
		}
		name += "." + next.text
	}
	if p.match(tokenSymbol, "<") {
		var args []string
		for {
			arg, err := p.parseTypeName()
			if err != nil {
				return "", nil, err
			}
			args = append(args, arg)
			if p.match(tokenSymbol, ">") {
				break
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return "", nil, err
			}
		}
		name += "<" + strings.Join(args, ",") + ">"
	}
	if p.match(tokenSymbol, "[") {
		if p.match(tokenSymbol, "]") {
			return name + "[]", nil, nil
		}
		size, err := p.parseExpression()
		if err != nil {
			return "", nil, err
		}
		if _, err := p.expect(tokenSymbol, "]"); err != nil {
			return "", nil, err
		}
		return name + "[]", &size, nil
	}
	return name, nil, nil
}

func (p *parser) parseCastExpression() (ir.Expr, bool, error) {
	save := p.pos
	typeName, err := p.parseTypeName()
	if err != nil {
		p.pos = save
		return ir.Expr{}, false, nil
	}
	if !p.match(tokenSymbol, ")") {
		p.pos = save
		return ir.Expr{}, false, nil
	}
	if !p.startsExpression() {
		p.pos = save
		return ir.Expr{}, false, nil
	}
	expr, err := p.parseUnary()
	if err != nil {
		return ir.Expr{}, true, err
	}
	return ir.Expr{Kind: ir.ExprCall, Callee: "__cast:" + typeName, Args: []ir.Expr{expr}}, true, nil
}

func (p *parser) startsExpression() bool {
	tok := p.tokens[p.pos]
	if tok.kind == tokenNumber || tok.kind == tokenString || tok.kind == tokenIdent {
		return true
	}
	return tok.kind == tokenSymbol && (tok.text == "(" || tok.text == "[" || tok.text == "!" || tok.text == "-")
}

func (p *parser) parseNewArgs() ([]ir.Expr, []ir.NamedArg, bool, error) {
	switch {
	case p.match(tokenSymbol, "("):
		if p.match(tokenSymbol, ")") {
			return nil, nil, false, nil
		}
		var args []ir.Expr
		var named []ir.NamedArg
		for {
			if p.peek(tokenIdent, "") && p.peekNext(tokenSymbol, "=") {
				name := p.advance().text
				p.advance()
				expr, err := p.parseExpression()
				if err != nil {
					return nil, nil, false, err
				}
				named = append(named, ir.NamedArg{Name: name, Expr: expr})
			} else {
				expr, err := p.parseExpression()
				if err != nil {
					return nil, nil, false, err
				}
				args = append(args, expr)
			}
			if p.match(tokenSymbol, ")") {
				return args, named, false, nil
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return nil, nil, false, err
			}
		}
	case p.match(tokenSymbol, "{"):
		if p.match(tokenSymbol, "}") {
			return nil, nil, true, nil
		}
		var args []ir.Expr
		for {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, nil, false, err
			}
			if p.match(tokenSymbol, "=") {
				if _, err := p.expect(tokenSymbol, ">"); err != nil {
					return nil, nil, false, err
				}
				value, err := p.parseExpression()
				if err != nil {
					return nil, nil, false, err
				}
				expr = ir.Expr{Kind: ir.ExprCall, Callee: "__mapEntry", Args: []ir.Expr{expr, value}}
			}
			args = append(args, expr)
			if p.match(tokenSymbol, "}") {
				return args, nil, true, nil
			}
			if _, err := p.expect(tokenSymbol, ","); err != nil {
				return nil, nil, false, err
			}
		}
	default:
		return nil, nil, false, fmt.Errorf("expected constructor arguments at byte %d", p.tokens[p.pos].pos)
	}
}

func parseNumberTokenText(text string) (string, bool) {
	if len(text) > 0 && strings.ContainsRune("LlDdFf", rune(text[len(text)-1])) {
		text = text[:len(text)-1]
	}
	return text, strings.ContainsAny(text, ".eE")
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
					return ir.Expr{Kind: ir.ExprSOQL, Value: strings.Join(compactSOQLDateLiteralParts(parts), " ")}, nil
				}
			}
		}
		if tok.kind == tokenString {
			parts = append(parts, soqlStringLiteralFromTokenText(tok.text))
		} else {
			parts = append(parts, tok.text)
		}
	}
	return ir.Expr{}, fmt.Errorf("unterminated SOQL literal at byte %d", pos)
}

func compactSOQLDateLiteralParts(parts []string) []string {
	if len(parts) < 5 {
		return parts
	}
	var out []string
	for i := 0; i < len(parts); {
		if i+4 < len(parts) &&
			isSOQLDateYearPart(parts[i]) &&
			parts[i+1] == "-" &&
			isSOQLDateMonthDayPart(parts[i+2]) &&
			parts[i+3] == "-" &&
			isSOQLDateMonthDayPart(parts[i+4]) {
			if out == nil {
				out = make([]string, 0, len(parts))
				out = append(out, parts[:i]...)
			}
			out = append(out, parts[i]+"-"+parts[i+2]+"-"+parts[i+4])
			i += 5
			continue
		}
		if out != nil {
			out = append(out, parts[i])
		}
		i++
	}
	if out == nil {
		return parts
	}
	return out
}

func isSOQLDateYearPart(part string) bool {
	return len(part) == 4 && isAllDigits(part)
}

func isSOQLDateMonthDayPart(part string) bool {
	return len(part) == 2 && isAllDigits(part)
}

func isAllDigits(part string) bool {
	for i := 0; i < len(part); i++ {
		if part[i] < '0' || part[i] > '9' {
			return false
		}
	}
	return part != ""
}

func soqlStringLiteralFromTokenText(text string) string {
	var out strings.Builder
	out.Grow(len(text) + 2)
	out.WriteByte('\'')
	for i := 0; i < len(text); i++ {
		if text[i] == '\'' {
			out.WriteString("''")
			continue
		}
		out.WriteByte(text[i])
	}
	out.WriteByte('\'')
	return out.String()
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
	if p.peek(tokenIdent, "new") {
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

func (p *parser) isDeclarationStartAfterFinal() bool {
	if !p.peek(tokenIdent, "final") {
		return false
	}
	save := p.pos
	p.advance()
	ok := p.isDeclarationStart()
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

func (p *parser) matchAnySymbol(symbols ...string) bool {
	for _, symbol := range symbols {
		if p.match(tokenSymbol, symbol) {
			return true
		}
	}
	return false
}

func (p *parser) peekAnySymbol(symbols ...string) bool {
	for _, symbol := range symbols {
		if p.peek(tokenSymbol, symbol) {
			return true
		}
	}
	return false
}

func (p *parser) peek(kind tokenKind, text string) bool {
	tok := p.tokens[p.pos]
	if tok.kind != kind {
		return false
	}
	return tokenTextMatches(kind, tok.text, text)
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
	return tokenTextMatches(kind, tok.text, text)
}

func tokenTextMatches(kind tokenKind, actual, expected string) bool {
	if expected == "" {
		return true
	}
	if kind == tokenIdent {
		return strings.EqualFold(actual, expected)
	}
	return actual == expected
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
