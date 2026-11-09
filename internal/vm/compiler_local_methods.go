package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/ir"
)

// anonymous_local_methods_c4 (API 62.0 and 67.0): an anonymous block may
// declare methods at its top level, before or after the statements that call
// them. Native compiles the block as type anon, every method static whatever
// its modifiers, so the methods also read and write the block's top-level
// variables.
var localMethodModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "global": true,
	"static": true, "override": true, "virtual": true, "final": true, "webservice": true,
}

var localMethodNonTypeWords = map[string]bool{
	"return": true, "throw": true, "new": true, "insert": true, "update": true, "delete": true,
	"upsert": true, "merge": true, "undelete": true, "if": true, "else": true, "for": true,
	"while": true, "do": true, "try": true, "catch": true, "finally": true, "switch": true,
	"break": true, "continue": true, "class": true, "interface": true, "enum": true,
}

// parseLocalMethodDeclaration parses one top-level method declaration:
// modifiers, a return type (or void), a name, a parameter list and a block.
// It leaves the position unchanged and reports false for any other statement.
func (p *parser) parseLocalMethodDeclaration() (ir.LocalMethod, bool, error) {
	save := p.pos
	start := p.tokens[p.pos].pos
	var modifiers []string
	for p.pos < len(p.tokens) && p.tokens[p.pos].kind == tokenIdent && localMethodModifiers[strings.ToLower(p.tokens[p.pos].text)] {
		modifiers = append(modifiers, strings.ToLower(p.tokens[p.pos].text))
		p.pos++
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].kind != tokenIdent || localMethodNonTypeWords[strings.ToLower(p.tokens[p.pos].text)] {
		p.pos = save
		return ir.LocalMethod{}, false, nil
	}
	returnType, err := p.parseTypeName()
	if err != nil || p.pos >= len(p.tokens) || p.tokens[p.pos].kind != tokenIdent || !p.peekNext(tokenSymbol, "(") {
		p.pos = save
		return ir.LocalMethod{}, false, nil
	}
	nameToken := p.advance()
	p.advance() // (
	var params []ir.LocalParam
	if !p.match(tokenSymbol, ")") {
		for {
			p.match(tokenIdent, "final")
			paramType, err := p.parseTypeName()
			if err != nil {
				p.pos = save
				return ir.LocalMethod{}, false, nil
			}
			paramName, err := p.expect(tokenIdent, "")
			if err != nil {
				p.pos = save
				return ir.LocalMethod{}, false, nil
			}
			params = append(params, ir.LocalParam{Name: paramName.text, Type: paramType})
			if p.match(tokenSymbol, ")") {
				break
			}
			if !p.match(tokenSymbol, ",") {
				p.pos = save
				return ir.LocalMethod{}, false, nil
			}
		}
	}
	if !p.peek(tokenSymbol, "{") {
		p.pos = save
		return ir.LocalMethod{}, false, nil
	}
	for _, modifier := range modifiers {
		if modifier == "webservice" {
			// anonymous_local_methods_c4 webservice: Ln 1, Col 27 (the name).
			message := "Defining type for webService methods must be declared as global"
			return ir.LocalMethod{}, true, &ApexSyntaxError{Message: message, NativeMessage: message, Offset: nameToken.pos}
		}
	}
	bodyStart := p.tokens[p.pos].pos
	body, err := p.parseStatementBlock()
	if err != nil {
		return ir.LocalMethod{}, true, err
	}
	bodyEnd := p.tokens[p.pos-1].pos + 1
	return ir.LocalMethod{Name: nameToken.text, ReturnType: returnType, Params: params, Modifiers: modifiers, Body: body, Pos: start, NamePos: nameToken.pos, BodyStart: bodyStart, BodyEnd: bodyEnd}, true, nil
}
