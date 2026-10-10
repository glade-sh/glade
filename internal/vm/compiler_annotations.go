package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexlang"
	"github.com/glade-sh/glade/internal/ir"
)

// Anonymous root variables are static fields of the implicit enclosing type.
// Their captured annotation admission is separate from class fields.
func (p *parser) parseAnnotatedDeclaration() (ir.Instruction, error) {
	start := p.tokens[p.pos].pos
	for p.match(tokenSymbol, "@") {
		name, err := p.expect(tokenIdent, "")
		if err != nil {
			return ir.Instruction{}, err
		}
		spec, known := apexlang.LookupAnnotation(name.text)
		annotationName := spec.Name
		if strings.EqualFold(annotationName, "future") {
			annotationName = "Future"
		}
		var argumentTokens []token
		if p.match(tokenSymbol, "(") {
			for !p.peek(tokenSymbol, ")") && !p.peek(tokenEOF, "") {
				argumentTokens = append(argumentTokens, p.advance())
			}
			if _, err := p.expect(tokenSymbol, ")"); err != nil {
				return ir.Instruction{}, err
			}
		}
		message := ""
		if !known {
			message = "unknown Apex annotation @" + name.text
		} else {
			switch strings.ToLower(annotationName) {
			case "testvisible", "deprecated":
			case "suppresswarnings":
				if len(argumentTokens) > 0 && (len(argumentTokens) != 1 || argumentTokens[0].kind != tokenString) {
					message = "Unexpected token '@'."
				}
			case "auraenabled":
				message = "AuraEnabled fields require at least one of the following global, public"
			case "invocablevariable":
				message = "InvocableVariable fields cannot be static"
			case "namespaceaccessible":
				message = "NamespaceAccessible fields require at least one of the following public, protected"
			case "testsetup":
				message = annotationName + " is not allowed on fields"
				// An anonymous method declaration has its own annotation contract.
				// Keep the captured field annotation contract intact.
				if p.pos+2 < len(p.tokens) && p.tokens[p.pos].text == "void" && p.tokens[p.pos+1].kind == tokenIdent && p.tokens[p.pos+2].text == "(" {
					message = "Defining type for TestSetup methods must be declared as IsTest"
				}
			default:
				message = annotationName + " is not allowed on fields"
			}
		}
		if message != "" {
			return ir.Instruction{}, &ApexSyntaxError{Message: message, Offset: start}
		}
	}
	if p.peek(tokenIdent, "final") && p.isDeclarationStartAfterFinal() {
		p.advance()
	}
	if !p.isDeclarationStart() {
		return ir.Instruction{}, &ApexSyntaxError{Message: "Unexpected token '@'.", Offset: start}
	}
	return p.parseDeclaration(start)
}
