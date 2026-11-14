package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
)

type receiverEnclosure struct {
	kind       string
	start      int
	semicolons int
	header     int
	doBlock    bool
	classBody  bool
}

// receiverBoundDiagnostic classifies the parser's enclosing construct for a
// bound name dot. Unmeasured constructs are left to the existing parser.
func receiverBoundDiagnostic(source, file, route string, item receiverTrivia, resolvedCall func() bool) *diagnostic.Diagnostic {
	if item.kind != receiverBoundBreak {
		return nil
	}
	ts, dot := item.tokens, item.dot
	if dot+1 >= len(ts) {
		return nil
	}
	if len(ts[dot+1].text) == 0 || !isApexIdentifierChar(ts[dot+1].text[0]) || ts[dot+1].text[0] >= '0' && ts[dot+1].text[0] <= '9' {
		return nil
	}
	stack, statement, blockDepth := item.stack, item.statement, item.blockDepth
	if statement >= len(ts) {
		return nil
	}
	member := ts[dot+1]
	report := func(token receiverTriviaToken, message string) *diagnostic.Diagnostic {
		return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, File: file, Range: receiverTriviaRange(source, token.start, token.end)}
	}
	topLevel := route == "anonymous" && blockDepth == 0 || route == "trigger" && blockDepth == 1
	leading := strings.ToLower(ts[statement].text)
	// Uncaptured neighboring constructs retain the ordinary parser result.
	if len(stack) > 0 && stack[len(stack)-1].kind == "call" {
		// Only the first-argument trigger form has a captured diagnostic.
		if route == "trigger" && dot >= 2 && ts[dot-2].text == "," && strings.EqualFold(ts[dot-1].text, "Trigger") && strings.EqualFold(member.text, "new") {
			return nil
		}
		if receiverTernaryArm(ts, stack, statement, dot) == 2 {
			return nil
		}
	}
	if leading == "when" {
		end := receiverDirectCallEnd(ts, dot)
		if receiverNameRoot(ts, dot) != statement+1 || end < 0 || end >= len(ts) || ts[end].text != "{" || !resolvedCall() {
			return nil
		}
	}
	switch leading {
	case "if", "for", "while", "switch", "return", "throw", "else":
		topLevel = false
	}
	// The anonymous parser reads a local method declaration as one top-level
	// member; a break inside it is reported where that member's name starts.
	if !topLevel && route == "anonymous" && len(stack) > 0 && stack[0].kind == "block" {
		if tok, ok := receiverLocalMethodToken(ts, stack[0]); ok {
			if leading == "return" {
				return nil
			}
			return report(tok, "Unexpected token '"+tok.text+"'.")
		}
	}
	if !topLevel && leading == "when" && dot+2 < len(ts) && ts[dot+2].text == "(" && (len(stack) == 0 || stack[len(stack)-1].kind == "block") {
		return report(ts[dot+2], "Missing '{' at '('")
	}
	if !topLevel {
		for i := statement; i < dot; i++ {
			if strings.EqualFold(ts[i].text, "instanceof") && i+1 == receiverNameRoot(ts, dot) && dot+2 < len(ts) && ts[dot+2].text == ";" {
				return report(member, "Extra ';', at '"+member.text+"'.")
			}
		}
		switch receiverTernaryArm(ts, stack, statement, dot) {
		case 1:
			return report(member, "Missing ':' at '"+member.text+"'")
		case 2:
			// The captured false arm is not yet distinguished from accepted
			// neighbors, so retain the previous diagnostic behavior.
			return nil
		}
	}
	if !topLevel {
		genericDepth := 0
		for i := statement; i < dot; i++ {
			if receiverGenericOpen(ts, i) && i+1 < dot {
				genericDepth++
			} else if ts[i].text == ">" && genericDepth > 0 {
				genericDepth--
			}
		}
		if genericDepth > 0 {
			if statement+1 < len(ts) && ts[statement+1].text == "<" {
				return report(ts[statement], "Unexpected token '"+ts[statement].text+"'.")
			}
			return nil
		}
	}
	if topLevel {
		first := ts[statement]
		if !first.word {
			return report(first, "Unexpected token '"+first.text+"'.")
		}
		end := statement
		for end+2 < len(ts) && ts[end+1].text == "." && ts[end].end == ts[end+1].start && ts[end+1].end == ts[end+2].start && ts[end+2].word {
			end += 2
		}
		if end+1 < len(ts) && ts[end+1].text == "." && ts[end].end == ts[end+1].start {
			end++
		}
		// A simple name-dot statement that calls a zero-argument method or
		// assigns a value is parsed as a broken type declaration.
		if dot == statement+1 && end == dot && dot+2 < len(ts) {
			next := ts[dot+2].text
			if next == "=" {
				return report(first, "Invalid type: "+first.text+".")
			}
			if next == "(" && dot+4 < len(ts) && ts[dot+3].text == ")" && ts[dot+4].text == ";" {
				return report(member, "Invalid type: "+first.text+".")
			}
		}
		if end+1 < len(ts) {
			tok := ts[end+1]
			return report(tok, "Unexpected token '"+tok.text+"'.")
		}
		return nil
	}
	for i := len(stack) - 1; i >= 0; i-- {
		frame := stack[i]
		switch frame.kind {
		case "block":
			i = -1
		case "call", "group", "new":
			if frame.kind == "group" && dot+2 < len(ts) && ts[dot+2].text == ")" {
				if dot+3 < len(ts) && (ts[dot+3].word || strings.HasPrefix(ts[dot+3].text, "'") || ts[dot+3].text == ";" && statement+1 < frame.start && ts[frame.start-1].text == "=") {
					return report(member, "Extra ')', at '"+member.text+"'.")
				}
				return nil
			}
			if route == "trigger" && dot > 0 && strings.EqualFold(ts[dot-1].text, "Trigger") && strings.EqualFold(member.text, "old") {
				return report(member, "Extra ')', at '"+member.text+"'.")
			}
			return report(member, "Expecting ')' but was: '"+member.text+"'")
		case "if", "while":
			if frame.kind == "if" && frame.start >= 2 && strings.EqualFold(ts[frame.start-2].text, "else") && (!receiverDirectCallEquality(ts, frame.start, dot) || !resolvedCall()) {
				return nil
			}
			return report(member, "Missing ')' at '"+member.text+"'")
		case "dowhile":
			if receiverDirectCallEquality(ts, frame.start, dot) && resolvedCall() {
				return report(member, "Expecting ')' but was: '"+member.text+"'")
			}
			return nil
		case "for":
			if frame.semicolons == 0 {
				if receiverForEach(ts, frame.start, dot) {
					if route == "trigger" && dot > 0 && strings.EqualFold(ts[dot-1].text, "Trigger") && strings.EqualFold(member.text, "new") {
						return report(member, "Extra ')', at '"+member.text+"'.")
					}
					end := receiverDirectCallEnd(ts, dot)
					if receiverNameRoot(ts, dot) != frame.start+4 || !ts[frame.start+1].word || !ts[frame.start+2].word || ts[frame.start+3].text != ":" || end < 0 || end+5 >= len(ts) || ts[end].text != "." || !ts[end+1].word || ts[end+2].text != "(" || !strings.HasPrefix(ts[end+3].text, "'") || ts[end+4].text != ")" || ts[end+5].text != ")" || !resolvedCall() {
						return nil
					}
					return report(member, "Missing ')' at '"+member.text+"'")
				}
				root := receiverNameRoot(ts, dot)
				end := receiverDirectCallEnd(ts, dot)
				if root != frame.start+4 || !ts[frame.start+1].word || !ts[frame.start+2].word || ts[frame.start+3].text != "=" || end < 0 || end >= len(ts) || ts[end].text != ";" || !resolvedCall() {
					return nil
				}
				return report(member, "Missing ';' at '"+member.text+"'")
			}
			if frame.semicolons == 1 {
				return report(member, "Missing ';' at '"+member.text+"'")
			}
			if frame.semicolons == 2 {
				return report(member, "Missing ')' at '"+member.text+"'")
			}
			return nil
		case "index":
			return report(member, "Expecting ']' but was: '"+member.text+"'")
		case "initializer":
			return report(member, "Expecting '}' but was: '"+member.text+"'")
		case "switch":
			return nil
		}
	}
	if leading == "switch" {
		return report(member, "Expecting '{' but was: '"+member.text+"'")
	}
	if route == "trigger" && dot > 0 && strings.EqualFold(ts[dot-1].text, "Trigger") && strings.EqualFold(member.text, "new") {
		return report(member, "Extra ';', at '"+member.text+"'.")
	}
	return report(member, "Missing ';' at '"+member.text+"'")
}

// Only a resolved zero-argument static call has been captured in these
// control expressions. Unknown and invalid calls keep ordinary diagnostics.
func receiverResolvedDirectCall(source string, item receiverTrivia, model *semaTypeMemberView) bool {
	ts, dot := item.tokens, item.dot
	if receiverDirectCallEnd(ts, dot) < 0 {
		return false
	}
	root := receiverNameRoot(ts, dot)
	if root < 0 || dot <= root {
		return false
	}
	owner := source[ts[root].start:ts[dot-1].end]
	candidates := preferResolvedMethodsByReceiverMode(resolveMemberMethods(model, owner, ts[dot+1].text), "class")
	member, ok, ambiguous := bestResolvedMemberByArgTypes(candidates, nil, model)
	return ok && !ambiguous && hasModifier(member.member.Modifiers, "static")
}

// receiverDirectCallEnd recognizes the captured zero-argument call.
func receiverDirectCallEnd(ts []receiverTriviaToken, dot int) int {
	if dot+3 < len(ts) && ts[dot+2].text == "(" && ts[dot+3].text == ")" {
		return dot + 4
	}
	return -1
}

// The captured condition compares the direct call to a string literal.
func receiverDirectCallEquality(ts []receiverTriviaToken, open, dot int) bool {
	if receiverNameRoot(ts, dot) != open+1 {
		return false
	}
	end := receiverDirectCallEnd(ts, dot)
	if end < 0 || end+3 >= len(ts) {
		return false
	}
	literal := ts[end+2].text
	return ts[end].text == "=" && ts[end+1].text == "=" && ts[end].end == ts[end+1].start && len(literal) >= 2 && strings.HasPrefix(literal, "'") && strings.HasSuffix(literal, "'") && ts[end+3].text == ")"
}

// receiverTernaryArm reports whether the break sits directly in a conditional
// expression of its innermost enclosing construct: 1 in the true arm, before
// the colon; 2 in the false arm; 0 otherwise. A conditional that closed inside
// nested brackets or before a separator does not enclose the break.
func receiverTernaryArm(ts []receiverTriviaToken, stack []receiverEnclosure, statement, dot int) int {
	from := statement
	if n := len(stack); n > 0 && stack[n-1].kind != "block" && stack[n-1].start+1 > from {
		from = stack[n-1].start + 1
	}
	open, closed, nested := 0, false, 0
	for i := from; i < dot; i++ {
		switch ts[i].text {
		case "(", "[", "{":
			nested++
		case ")", "]", "}":
			nested--
		}
		if nested != 0 {
			continue
		}
		switch ts[i].text {
		case "?":
			if i+1 >= len(ts) || ts[i+1].text != "." || ts[i].end != ts[i+1].start {
				open++
			}
		case ":":
			if open > 0 {
				open--
				closed = true
			}
		case ",", ";":
			open, closed = 0, false
		}
	}
	if open > 0 {
		return 1
	}
	if closed {
		return 2
	}
	return 0
}

// receiverLocalMethodParameter reports whether the name left of a free dot is
// a parameter of the anonymous local method that contains it. A parameter is
// a value there, including one that shares a type's name.
func receiverLocalMethodParameter(program ir.Program, item receiverTrivia) bool {
	if item.kind != receiverFreeName {
		return false
	}
	head := item.tokens[receiverNameRoot(item.tokens, item.dot)]
	for _, local := range program.LocalMethods {
		if head.start <= local.BodyStart || head.start >= local.BodyEnd {
			continue
		}
		for _, param := range local.Params {
			if strings.EqualFold(param.Name, head.text) {
				return true
			}
		}
	}
	return false
}

// receiverNameRoot finds the first segment of the actual name operand.
func receiverNameRoot(ts []receiverTriviaToken, dot int) int {
	root := dot - 1
	for root >= 2 && ts[root-1].text == "." && ts[root-2].word && ts[root-2].end == ts[root-1].start && ts[root-1].end == ts[root].start {
		root -= 2
	}
	return root
}

// A generic argument list must close before an expression operator or a
// statement boundary. In particular, a comparison does not open a type.
func receiverGenericOpen(ts []receiverTriviaToken, i int) bool {
	if i <= 0 || i+1 >= len(ts) || ts[i].text != "<" || !ts[i-1].word || !ts[i+1].word {
		return false
	}
	depth := 1
	for j := i + 1; j < len(ts); j++ {
		switch ts[j].text {
		case "<":
			depth++
		case ">":
			depth--
			if depth == 0 {
				return true
			}
		case ";", "{", "}", "=", "&", "|", "?", ":", "(", ")", "+", "-", "*":
			return false
		}
	}
	return false
}

// receiverFreeDiagnostic resolves the complete name path to the left of a
// detached dot as a value, using the ordinary scope and type model.
func receiverFreeDiagnostic(source, namespace string, item receiverTrivia, scopes semaScopeModel, model *semaTypeMemberView) *diagnostic.Diagnostic {
	return receiverFreeDiagnosticAt(source, namespace, item, scopes, model, 0)
}

func receiverFreeDiagnosticAt(source, namespace string, item receiverTrivia, scopes semaScopeModel, model *semaTypeMemberView, bodyOffset int) *diagnostic.Diagnostic {
	if item.kind != receiverFreeName {
		return nil
	}
	ts, dot := item.tokens, item.dot
	if dot+1 >= len(ts) || strings.EqualFold(ts[dot+1].text, "class") {
		return nil
	}
	root := receiverNameRoot(ts, dot)
	if root > 0 && ts[root-1].text == "." {
		return nil
	}
	report := func(token receiverTriviaToken, message string) *diagnostic.Diagnostic {
		return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, Range: receiverTriviaRange(source, token.start, token.end)}
	}
	// These type positions have their own native parser messages.
	if root > 0 && receiverGenericOpen(ts, root-1) {
		if item.statement+1 < len(ts) && ts[item.statement+1].text == "<" {
			lead := ts[item.statement]
			return report(lead, "Unexpected token '"+lead.text+"'.")
		}
		return nil
	}
	if root > 0 && ts[root-1].text == "(" && dot+3 < len(ts) && ts[dot+2].text == ")" {
		literal := ts[dot+3]
		if len(literal.text) >= 2 && strings.HasPrefix(literal.text, "'") && strings.HasSuffix(literal.text, "'") {
			return report(literal, "Extra ';', at '"+literal.text[1:len(literal.text)-1]+"'.")
		}
		// A delimiter after the closing parenthesis completes an argument or
		// grouped expression; it cannot be a cast followed by an operand.
		if literal.text != ";" && literal.text != "," && literal.text != ")" && literal.text != "]" && literal.text != "}" {
			return nil
		}
	}
	for i := root - 1; i >= 0; i-- {
		if ts[i].text == ";" || ts[i].text == "{" || ts[i].text == "}" {
			break
		}
		if strings.EqualFold(ts[i].text, "instanceof") && i+1 == root {
			if dot+2 < len(ts) && ts[dot+2].text == ";" {
				return report(ts[dot], "Expecting ';' but was: '.'")
			}
			return nil
		}
		if strings.EqualFold(ts[i].text, "extends") || strings.EqualFold(ts[i].text, "implements") {
			return nil
		}
	}
	head := ts[root]
	if strings.EqualFold(head.text, "this") || strings.EqualFold(head.text, "super") {
		return nil
	}
	if _, ok := scopes.visibleAt(head.text, head.start-bodyOffset); ok {
		return nil
	}
	path := source[head.start:ts[dot-1].end]
	isType := semaIRExprLooksLikeTypeReceiver(ir.Expr{Kind: ir.ExprVariable, Name: path}, newIRSemaScope(nil), model)
	isNamespace := strings.EqualFold(path, "System") || namespace != "" && strings.EqualFold(path, namespace)
	if !isType && !isNamespace {
		return nil
	}
	// A detached dot in a type declaration cannot join the type name.
	if dot+2 < len(ts) && ts[dot+1].word && ts[dot+2].word {
		if item.depth == 0 {
			if tok, ok := receiverMethodHeaderToken(ts, item.statement, dot); ok && item.statement != root {
				return report(tok, "Unexpected token '"+tok.text+"'.")
			}
			return report(ts[dot], "Unexpected token '.'.")
		}
		return report(ts[dot+2], "Missing ';' at '"+ts[dot+2].text+"'")
	}
	// `new Type .Inner()` is a broken type, not an expression receiver.
	if root > 0 && strings.EqualFold(ts[root-1].text, "new") {
		statement := item.statement
		if statement >= len(ts) {
			return nil
		}
		first := ts[statement]
		if !first.word {
			return report(first, "Unexpected token '"+first.text+"'.")
		}
		if statement+1 < len(ts) {
			token := ts[statement+1]
			return report(token, "Unexpected token '"+token.text+"'.")
		}
		return nil
	}
	return report(head, "Variable does not exist: "+head.text)
}

// Salesforce counts a lone carriage return as a line break. Keep this local
// to receiver diagnostics rather than changing unrelated source ranges.
func receiverTriviaRange(source string, start, end int) *diagnostic.Range {
	if start < 0 || end < start || start > len(source) {
		return nil
	}
	if end > len(source) {
		end = len(source)
	}
	if !strings.Contains(source[:end], "\r") {
		return semaRange(source, start, end)
	}
	position := func(offset int) diagnostic.Position {
		line, column := 1, 1
		for i := 0; i < offset; i++ {
			switch source[i] {
			case '\r':
				line++
				column = 1
				if i+1 < offset && source[i+1] == '\n' {
					i++
				}
			case '\n':
				line++
				column = 1
			default:
				column++
			}
		}
		return diagnostic.Position{Line: line, Column: column, Offset: offset}
	}
	return &diagnostic.Range{Start: position(start), End: position(end)}
}

// receiverControlDiagnostic covers the two observed top-level call shapes.
// The vertical tab is an invalid source character, not trivia.
func receiverControlDiagnostic(source string, item receiverTrivia) *diagnostic.Diagnostic {
	if item.kind != receiverInvalidControl || item.blockDepth != 0 {
		return nil
	}
	ts, i := item.tokens, item.dot
	bound := i >= 2 && i+2 < len(ts) && item.statement == i-2 && ts[i-2].word && ts[i-1].text == "." && ts[i-2].end == ts[i-1].start && ts[i-1].end == ts[i].start && ts[i+1].word && ts[i].end == ts[i+1].start && ts[i+2].text == "("
	free := i >= 1 && i+3 < len(ts) && item.statement == i-1 && ts[i-1].word && ts[i-1].end == ts[i].start && ts[i+1].text == "." && ts[i].end == ts[i+1].start && ts[i+2].word && ts[i+1].end == ts[i+2].start && ts[i+3].text == "("
	if !bound && !free {
		return nil
	}
	token := ts[i]
	const message = "Found control character hex 0x000B (decimal 11) that isn't valid in Apex."
	return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, Range: receiverTriviaRange(source, token.start, token.end)}
}

// receiverForEach reports whether the for header opened at index open holds a
// colon outside any conditional expression before the dot: a for-each header.
func receiverForEach(ts []receiverTriviaToken, open, dot int) bool {
	ternary, nested := 0, 0
	for i := open + 1; i < dot; i++ {
		switch ts[i].text {
		case "(", "[", "{":
			nested++
		case ")", "]", "}":
			nested--
		}
		if nested != 0 {
			continue
		}
		switch ts[i].text {
		case "?":
			if i+1 >= len(ts) || ts[i+1].text != "." || ts[i].end != ts[i+1].start {
				ternary++
			}
		case ":":
			if ternary == 0 {
				return true
			}
			ternary--
		}
	}
	return false
}

// receiverLocalMethodToken finds the token after the leading type name of the
// local method whose body is the given outermost block, if that block is one.
func receiverLocalMethodToken(ts []receiverTriviaToken, block receiverEnclosure) (receiverTriviaToken, bool) {
	b, lead := block.start, block.header
	if b < 1 || lead >= b || ts[b-1].text != ")" || !ts[lead].word {
		return receiverTriviaToken{}, false
	}
	depth, open := 0, -1
	for i := b - 1; i > lead; i-- {
		if ts[i].text == ")" {
			depth++
		} else if ts[i].text == "(" {
			depth--
			if depth == 0 {
				open = i
				break
			}
		}
	}
	if open < lead+2 || !ts[open-1].word {
		return receiverTriviaToken{}, false
	}
	end := lead
	for end+2 < len(ts) && ts[end+1].text == "." && ts[end].end == ts[end+1].start && ts[end+1].end == ts[end+2].start && ts[end+2].word {
		end += 2
	}
	if end+1 < len(ts) && ts[end+1].text == "." && ts[end].end == ts[end+1].start {
		end++
	}
	if end+1 >= open {
		return receiverTriviaToken{}, false
	}
	return ts[end+1], true
}

// receiverMethodHeaderToken is the token after the leading name lexeme of a
// statement whose next tokens open a parameter list: `Type name (`.
func receiverMethodHeaderToken(ts []receiverTriviaToken, statement, dot int) (receiverTriviaToken, bool) {
	if statement >= len(ts) || !ts[statement].word {
		return receiverTriviaToken{}, false
	}
	end := statement
	for end+2 < len(ts) && ts[end+1].text == "." && ts[end].end == ts[end+1].start && ts[end+1].end == ts[end+2].start && ts[end+2].word {
		end += 2
	}
	if end+3 < len(ts) && end+1 < dot && ts[end+1].word && ts[end+2].text == "(" {
		return ts[end+1], true
	}
	return receiverTriviaToken{}, false
}
