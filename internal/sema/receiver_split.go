package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native anonymous Apex at API 62.0 and 67.0 rejects some calls whose
// receiver is split from its method by trivia. Glade reports these only for
// statements whose canonical shape equals a captured row. Any other shape
// keeps the existing analysis.
//
// The canonical shape keeps every token and the kind of every gap. It
// abstracts only the split call's receiver (R) and method (M), a declared
// local's name (V), literal contents (S, N), comment text and the width of
// horizontal whitespace. Identifiers are compared case-insensitively.
type splitReceiverRule struct {
	typeReceiver bool   // receiver is a class (true) or a local variable
	message      string // native message kind
}

const (
	splitReceiverVariable  = "variable" // Variable does not exist: <root>
	splitReceiverType      = "type"     // Invalid type: <receiver>.
	splitReceiverLocal     = "local"    // Unexpected token '<V>'.
	splitReceiverOpenParen = "paren"    // Unexpected token '('.
)

// splitReceiverShapes lists the canonical form of each rejected capture in
// testdata/receiver_split (org.tsv) and of the supplied chain-recv rows. The
// native test checks that this table and the fixtures agree exactly.
var splitReceiverShapes = map[string]splitReceiverRule{
	// static_comment, static_inline_comment, static_blank, static_space
	"system.assertequals(S, R /**/\n.M());": {true, splitReceiverVariable},
	"system.assertequals(S, R /**/ .M());":  {true, splitReceiverVariable},
	"system.assertequals(S, R\n\n.M());":    {true, splitReceiverVariable},
	"system.assertequals(S, R . M());":      {true, splitReceiverVariable},
	// qualified_split
	"system.assertequals(S, system.string\n.M(N));": {true, splitReceiverVariable},
	// split_statement and the supplied chain-recv shape
	"R /**/ .M();": {true, splitReceiverVariable},
	"R\n.M();":     {true, splitReceiverVariable},
	// repeat_first, repeat_second, repeat_both
	"system.assertequals(R\n.M(), R.M());":   {true, splitReceiverVariable},
	"system.assertequals(R.M(), R\n.M());":   {true, splitReceiverVariable},
	"system.assertequals(R\n.M(), R\n.M());": {true, splitReceiverVariable},
	// static_dot_before, comment_after, comment_before_newline, nested
	"system.assertequals(S, R.\nM());":                 {true, splitReceiverOpenParen},
	"system.assertequals(S, R./*\n*/M());":             {true, splitReceiverOpenParen},
	"system.assertequals(S, R./**/\nM());":             {true, splitReceiverOpenParen},
	"system.assertequals(S, string.valueof(R.\nM()));": {true, splitReceiverOpenParen},
	// after_first, after_second, after_both
	"system.assertequals(R.\nM(), R.M());":   {true, splitReceiverOpenParen},
	"system.assertequals(R.M(), R.\nM());":   {true, splitReceiverOpenParen},
	"system.assertequals(R.\nM(), R.\nM());": {true, splitReceiverOpenParen},
	// dot_statement, comment_statement, dot_assign, comment_assignment
	"R.\nM();":                {true, splitReceiverType},
	"R./*\n*/M();":            {true, splitReceiverType},
	"string V = R.\nM();":     {true, splitReceiverLocal},
	"string V = R./*\n*/M();": {true, splitReceiverLocal},
	// dot_instance, instance_arg, instance_assign
	"R.\nM();\x00local":                         {false, splitReceiverType},
	"system.assertequals(S, R.\nM());\x00local": {false, splitReceiverOpenParen},
	"string V = R.\nM();\x00local":              {false, splitReceiverLocal},
}

type splitReceiverToken struct {
	text       string // token text
	gap        string // canonical gap before the token
	start, end int    // offsets in source
}

// splitReceiverGap canonicalizes trivia. A run of whitespace becomes one
// space, or its line breaks when it has any; a comment keeps only its kind
// and its line breaks.
func splitReceiverGap(trivia string) string {
	var b strings.Builder
	for i := 0; i < len(trivia); {
		switch {
		case strings.HasPrefix(trivia[i:], "//"):
			n := strings.IndexByte(trivia[i:], '\n')
			if n < 0 {
				n = len(trivia) - i
			}
			b.WriteString("//")
			i += n
		case strings.HasPrefix(trivia[i:], "/*"):
			n := strings.Index(trivia[i+2:], "*/") + 4
			b.WriteString("/*" + strings.Repeat("\n", strings.Count(trivia[i:i+n], "\n")) + "*/")
			i += n
		default:
			j := i
			for j < len(trivia) && !strings.HasPrefix(trivia[j:], "//") && !strings.HasPrefix(trivia[j:], "/*") {
				j++
			}
			if lines := strings.Count(trivia[i:j], "\n"); lines > 0 {
				b.WriteString(strings.Repeat("\n", lines))
			} else {
				b.WriteByte(' ')
			}
			i = j
		}
	}
	return b.String()
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
		gap := splitReceiverGap(source[start:i])
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

// splitReceiverShape renders the canonical shape of a statement for one
// receiver and method, and returns the first split occurrence's offset.
func splitReceiverShape(tokens []splitReceiverToken, receiver, method string) (string, int, bool) {
	parts := strings.Split(strings.ToLower(receiver), ".")
	method = strings.ToLower(method)
	var b strings.Builder
	at, found := -1, false
	for i := 0; i < len(tokens); i++ {
		n := 2*len(parts) + 1 // receiver parts and dots, then the method
		if i+n < len(tokens) && tokens[i+n].text == "(" && tokens[i+n-1].text == method {
			match := true
			for k, part := range parts {
				if tokens[i+2*k].text != part || tokens[i+2*k+1].text != "." {
					match = false
					break
				}
			}
			if match {
				// A qualified receiver keeps its spelling: only the
				// captured qualified class is matched.
				name := "R"
				if len(parts) > 1 {
					name = parts[0]
				}
				b.WriteString(tokens[i].gap + name)
				for k := 1; k < len(parts); k++ {
					b.WriteString(tokens[i+2*k-1].gap + "." + tokens[i+2*k].gap + parts[k])
				}
				dot, m := tokens[i+n-2], tokens[i+n-1]
				b.WriteString(dot.gap + "." + m.gap + "M")
				if !found && (dot.gap != "" || m.gap != "") {
					at, found = tokens[i].start, true
				}
				i += n - 1
				continue
			}
		}
		text := tokens[i].text
		if i == 1 && len(tokens) > 2 && tokens[2].text == "=" && isApexIdentifierChar(text[0]) {
			text = "V"
		}
		b.WriteString(tokens[i].gap + text)
	}
	return b.String(), at, found
}

func semaSplitStaticReceiver(typ typesys.TypeSymbol, expr ir.Expr, scope irSemaScope, pos int, source string, model *semaTypeMemberView) *diagnostic.Diagnostic {
	if !scope.anonymous || expr.Left != nil || pos < 0 || pos >= len(source) {
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
	// Every captured shape has trivia inside some occurrence of the callee
	// in this statement; skip the statement lexer otherwise.
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
	shape, at, ok := splitReceiverShape(tokens, receiver, method)
	if !ok {
		return nil
	}
	root, _, _ := strings.Cut(receiver, ".")
	_, local := scope.lookup(root)
	if local {
		shape += "\x00local"
	}
	rule, ok := splitReceiverShapes[shape]
	if !ok || rule.typeReceiver == local || (rule.typeReceiver && !semaReceiverExprLooksLikeType(receiver, scope, model)) || !splitReceiverTopLevel(source, tokens[0].start) {
		return nil
	}
	var message string
	switch rule.message {
	case splitReceiverVariable:
		message = "Variable does not exist: " + root
	case splitReceiverType:
		message = "Invalid type: " + receiver + "."
	case splitReceiverLocal:
		message = "Unexpected token '" + tokens[1].source(source) + "'."
	default:
		message = "Unexpected token '('."
	}
	return &diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESEMA_RECEIVER_SPLIT", Message: message, NativeMessage: message, File: typ.File, Range: semaRange(source, at, at+len(receiver))}
}

// splitReceiverTopLevel reports whether a top-level statement begins at
// start. Every capture is a top-level anonymous statement; statements in a
// block, or after anything the statement lexer cannot follow, are not.
func splitReceiverTopLevel(source string, start int) bool {
	for at := 0; at <= start; {
		tokens, ok := splitReceiverStatement(source, at)
		if !ok {
			return false
		}
		if tokens[0].start >= start {
			return tokens[0].start == start
		}
		at = tokens[len(tokens)-1].end
	}
	return false
}

func (t splitReceiverToken) source(source string) string { return source[t.start:t.end] }

// Repeated IR calls may identify the same source occurrence. Keep one report
// for that location without merging diagnostics at distinct locations.
func dedupeSplitReceiverDiagnostics(items []diagnostic.Diagnostic) []diagnostic.Diagnostic {
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
