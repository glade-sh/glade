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
}

// receiverBoundDiagnostic classifies the parser's enclosing construct for a
// bound name dot. Unmeasured constructs are left to the existing parser.
func receiverBoundDiagnostic(source, file, route string, item receiverTrivia) *diagnostic.Diagnostic {
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
	switch leading {
	case "if", "for", "while", "switch", "return", "throw":
		topLevel = false
	}
	if !topLevel {
		for i := statement; i < dot; i++ {
			if strings.EqualFold(ts[i].text, "instanceof") && dot+2 < len(ts) && ts[dot+2].text == ";" {
				return report(member, "Extra ';', at '"+member.text+"'.")
			}
			if ts[i].text == "?" && (i+1 >= len(ts) || ts[i+1].text != "." || ts[i].end != ts[i+1].start) {
				return report(member, "Missing ':' at '"+member.text+"'")
			}
		}
	}
	if !topLevel {
		genericDepth := 0
		for i := statement; i < dot; i++ {
			if ts[i].text == "<" && i > statement && ts[i-1].word && i+1 < dot && ts[i+1].word {
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
				if dot+3 < len(ts) && (ts[dot+3].word || strings.HasPrefix(ts[dot+3].text, "'")) {
					return report(member, "Extra ')', at '"+member.text+"'.")
				}
				return nil
			}
			if route == "trigger" && dot > 0 && strings.EqualFold(ts[dot-1].text, "Trigger") && strings.EqualFold(member.text, "old") {
				return report(member, "Extra ')', at '"+member.text+"'.")
			}
			return report(member, "Expecting ')' but was: '"+member.text+"'")
		case "if", "while":
			return report(member, "Missing ')' at '"+member.text+"'")
		case "for":
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
	return report(member, "Missing ';' at '"+member.text+"'")
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
	root := dot - 1
	for root >= 2 && ts[root-1].text == "." && ts[root-2].word && ts[root-2].end == ts[root-1].start && ts[root-1].end == ts[root].start {
		root -= 2
	}
	if root > 0 && ts[root-1].text == "." {
		return nil
	}
	report := func(token receiverTriviaToken, message string) *diagnostic.Diagnostic {
		return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA013", Message: message, NativeMessage: message, Range: receiverTriviaRange(source, token.start, token.end)}
	}
	// These type positions have their own native parser messages.
	if root > 0 && ts[root-1].text == "<" {
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
		if strings.EqualFold(ts[i].text, "instanceof") {
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
