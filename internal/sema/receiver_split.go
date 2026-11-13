package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

type splitReceiverToken struct {
	text       string // token text
	gap        string // source trivia before the token
	start, end int    // offsets in source
}

// splitReceiverStatement lexes one statement from pos through its first
// top-level semicolon. It reports false for anything it cannot lex.
func splitReceiverStatement(source string, pos int) ([]splitReceiverToken, bool) {
	var tokens []splitReceiverToken
	depth := 0
	for i := pos; i < len(source); {
		start := i
		for i < len(source) {
			if c := source[i]; c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' {
				i++
			} else if strings.HasPrefix(source[i:], "//") {
				n := strings.IndexByte(source[i:], '\n')
				if n < 0 {
					return nil, false
				}
				i += n
			} else if strings.HasPrefix(source[i:], "/*") {
				n := strings.Index(source[i+2:], "*/")
				if n < 0 {
					return nil, false
				}
				i += n + 4
			} else {
				break
			}
		}
		if i >= len(source) {
			return nil, false
		}
		gap := source[start:i]
		if len(tokens) == 0 {
			gap = ""
		}
		begin := i
		var text string
		switch c := source[i]; {
		case isApexIdentifierChar(c) && !(c >= '0' && c <= '9'):
			for i < len(source) && isApexIdentifierChar(source[i]) {
				i++
			}
			text = strings.ToLower(source[begin:i])
		case c >= '0' && c <= '9':
			for i < len(source) && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			text = "N"
		case c == '\'':
			i++
			for i < len(source) && source[i] != '\'' {
				if source[i] == '\\' {
					i++
				}
				if i < len(source) && source[i] == '\n' {
					return nil, false
				}
				i++
			}
			if i >= len(source) {
				return nil, false
			}
			i++
			text = "S"
		default:
			i++
			text = string(c)
			switch c {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
		tokens = append(tokens, splitReceiverToken{text: text, gap: gap, start: begin, end: i})
		if depth < 0 {
			return nil, false
		}
		if text == ";" && depth == 0 {
			return tokens, true
		}
	}
	return nil, false
}

// semaSplitStaticReceiver examines trivia at the matched call occurrence.
// Contexts without a captured diagnostic remain with the existing analyzer.
func semaSplitStaticReceiver(typ typesys.TypeSymbol, member typesys.MemberSymbol, expr ir.Expr, scope irSemaScope, pos int, source string, model *semaTypeMemberView) *diagnostic.Diagnostic {
	if expr.Left != nil || pos < 0 || pos >= len(source) {
		return nil
	}
	major, _ := apexversion.Major(typ.EffectiveAPIVersion)
	if major != 62 && major != 67 {
		return nil
	}
	receiver, method, ok := splitSemaMethodPath(expr.Callee)
	if !ok {
		return nil
	}
	// Most calls have no trivia in their name. Avoid lexing their statements.
	split := false
	for start, end, found := semaIRNameSpan(source, pos, expr.Callee); found; start, end, found = semaIRNameSpan(source, end, expr.Callee) {
		if end-start != len(expr.Callee) {
			split = true
			break
		}
	}
	if !split {
		return nil
	}
	tokens, ok := splitReceiverStatement(source, pos)
	if !ok {
		return nil
	}
	// The captures cover calls inside blocks, not a statement following one,
	// and do not cover a call in a return expression.
	if tokens[0].text == "return" || strings.HasSuffix(strings.TrimSpace(source[:pos]), "}") {
		return nil
	}
	// The named route is a class method body. Test classes and other hosts have
	// not been captured by this probe.
	if !scope.anonymous && (typ.IsTest || member.IsTest || !strings.HasSuffix(strings.ToLower(typ.File), ".cls")) {
		return nil
	}
	// Parsing errors after a dot take precedence over name resolution before it.
	hasAfter := false
	for i := 1; i+2 < len(tokens); i++ {
		if tokens[i].text == "." && tokens[i+1].gap != "" && tokens[i+2].text == "(" && tokens[i-1].text != ")" {
			hasAfter = true
		}
	}
	for at, end, found := semaIRNameSpan(source, pos, expr.Callee); found; at, end, found = semaIRNameSpan(source, end, expr.Callee) {
		first, last := -1, -1
		for i, t := range tokens {
			if t.start == at {
				first = i
			}
			if t.end == end {
				last = i
			}
		}
		if first < 0 || last <= first || last+1 >= len(tokens) || tokens[last+1].text != "(" {
			continue
		}
		root, _, _ := strings.Cut(receiver, ".")
		_, local := scope.lookup(root)
		before, after := false, false
		afterGap, beforeGap, beforeTrivia := "", "", ""
		for i := first + 1; i <= last; i++ {
			if tokens[i].text == "." {
				before = before || tokens[i].gap != ""
				beforeTrivia += tokens[i].gap
				if i == last-1 {
					beforeGap = tokens[i].gap
					afterGap = tokens[i+1].gap
					after = afterGap != ""
				}
			}
		}
		// Both sides split are captured only for a single space on each side.
		if before && after && !(beforeGap == " " && afterGap == " ") {
			continue
		}
		depth := 0
		for _, t := range tokens[:first] {
			if t.text == "(" {
				depth++
			}
			if t.text == ")" {
				depth--
			}
		}
		// Determine the surrounding argument and the token following this call.
		previousArgument, level := false, 0
		for _, token := range tokens[:first] {
			if token.text == "(" {
				level++
			}
			if token.text == ")" {
				level--
			}
			if token.text == "," && level == depth {
				previousArgument = true
			}
		}
		nesting, close := 1, last+2
		for close < len(tokens) && nesting > 0 {
			if tokens[close].text == "(" {
				nesting++
			}
			if tokens[close].text == ")" {
				nesting--
			}
			close++
		}
		// The captured declaration, expression and argument contexts have distinct
		// parser messages. Return and other expression contexts are not captured.
		stmt := first == 0
		decl := first >= 3 && len(tokens) > 3 && tokens[2].text == "="
		assign := first >= 2 && len(tokens) > 2 && tokens[1].text == "="
		arg := depth > 0 && depth <= 2 && tokens[0].text != "return" && tokens[0].text != "throw"
		if before {
			if assign {
				continue
			}
			if decl && tokens[last+2].text != ")" {
				continue
			}
			if stmt && tokens[last+2].text != ")" && (!strings.ContainsAny(beforeTrivia, "\r\n") || (close < len(tokens) && tokens[close].text == ".")) {
				continue
			}
			if depth > 0 && !previousArgument {
				laterArgument := close < len(tokens) && tokens[close].text == ","
				if (laterArgument && beforeTrivia != "\n") || (!laterArgument && (beforeGap != " " || after)) {
					continue
				}
			}
			if !scope.anonymous && depth > 0 && (!previousArgument || beforeTrivia != "\n") {
				continue
			}
			// Standalone calls are captured with a line break or comment;
			// declaration calls use a line break. Nested fluent and deeply
			// nested before-dot calls are unobserved.
			if decl && !strings.ContainsAny(beforeTrivia, "\r\n") {
				continue
			}
			if stmt && !strings.ContainsAny(beforeTrivia, "\r\n") && !strings.Contains(beforeTrivia, "/*") {
				continue
			}
			if depth > 1 || (depth > 0 && close < len(tokens) && tokens[close].text == ".") {
				continue
			}
			// A single-part receiver with call arguments is captured as a
			// statement, not inside an enclosing call.
			if depth > 0 && !strings.Contains(receiver, ".") && tokens[last+2].text != ")" {
				continue
			}
			// Two-sided trivia is observed in a later call argument only;
			// compound expressions with before-dot trivia are unobserved.
			if after && !previousArgument {
				continue
			}
			if close < len(tokens) && tokens[close].text != ")" && tokens[close].text != "," && tokens[close].text != ";" && tokens[close].text != "." {
				continue
			}
		}
		var message string
		if before && (!hasAfter || after) && !local && semaReceiverExprLooksLikeType(receiver, scope, model) {
			message = "Variable does not exist: " + root
		} else if after {
			// Assignment and binary argument captures use a line break after
			// the dot; space-only forms have no observed result.
			lineBreak := strings.ContainsAny(afterGap, "\r\n")
			if stmt && tokens[last+2].text != ")" {
				// The only captured argument-bearing standalone call is on
				// a platform receiver and has a line break after the dot.
				members, known := model.lookupName(receiver)
				if !known || !members.platform || !lineBreak {
					continue
				}
			}
			if ((decl || assign) || (arg && depth > 1)) && !lineBreak {
				continue
			}
			if arg && tokens[last+2].text != ")" {
				continue
			}
			if arg && depth == 1 && !previousArgument && (close >= len(tokens) || tokens[close].text != ",") {
				continue
			}
			if arg && close < len(tokens) && tokens[close].text != ")" && tokens[close].text != "," {
				// The only captured compound argument is a later argument
				// using + and a newline after the receiver dot.
				if !lineBreak || tokens[close].text != "+" || !previousArgument {
					continue
				}
			}

			if !stmt && !decl && !assign && !arg {
				continue
			}
			// The after-dot rows cover single-part receivers only.
			if strings.Contains(receiver, ".") {
				continue
			}
			// Local receivers with arguments and named fluent calls are unprobed.
			if local && (last+2 >= len(tokens) || tokens[last+2].text != ")") {
				continue
			}
			if !scope.anonymous && (afterGap != "\n" || !stmt || last+3 >= len(tokens) || tokens[last+2].text != ")" || tokens[last+3].text != ";") {
				continue
			}
			switch {
			case arg:
				message = "Unexpected token '('."
			case decl:
				message = "Unexpected token '" + tokens[1].source(source) + "'."
			case assign:
				message = "Unexpected token '='."
			case !scope.anonymous || len(scope.frames) > 1:
				message = "Missing ';' at '" + method + "'"
			case stmt:
				close := last + 2
				nesting := 1
				for close < len(tokens) && nesting > 0 {
					if tokens[close].text == "(" {
						nesting++
					}
					if tokens[close].text == ")" {
						nesting--
					}
					close++
				}
				fluent := close < len(tokens) && tokens[close].text == "."
				if fluent && local {
					continue // uncaptured local fluent receiver
				}
				if last+2 < len(tokens) && tokens[last+2].text != ")" || fluent {
					message = "Unexpected token '" + method + "'."
				} else {
					message = "Invalid type: " + receiver + "."
				}
			}
		}
		if message != "" {
			return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_RECEIVER_SPLIT", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, at, at+len(root))}
		}
	}
	return nil
}

func (t splitReceiverToken) source(source string) string { return source[t.start:t.end] }

// Repeated IR calls may identify the same source occurrence. Keep one report
// for that location without merging diagnostics at distinct locations.
func dedupeSplitReceiverDiagnostics(items []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	found := false
	for _, item := range items {
		if item.Code == "GLADESEMA_RECEIVER_SPLIT" {
			found = true
			break
		}
	}
	if !found {
		return items
	}
	seen := make(map[diagnostic.Position]map[string]bool)
	out := make([]diagnostic.Diagnostic, 0, len(items))
	for _, d := range items {
		if d.Code == "GLADESEMA_RECEIVER_SPLIT" && d.Range != nil {
			d.Code = "GLADESEMA013"
			pos := d.Range.Start
			if seen[pos] == nil {
				seen[pos] = make(map[string]bool)
			}
			if seen[pos][d.NativeMessage] {
				continue
			}
			seen[pos][d.NativeMessage] = true
		}
		out = append(out, d)
	}
	return out
}
