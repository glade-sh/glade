package sema

import "strings"

type receiverTriviaKind uint8

const (
	receiverBoundBreak receiverTriviaKind = iota + 1
	receiverFreeName
	receiverInvalidControl
)

type receiverTriviaToken struct {
	text       string
	start, end int
	word       bool
}

type receiverTrivia struct {
	kind                 receiverTriviaKind
	tokens               []receiverTriviaToken
	dot                  int
	statement            int
	blockDepth           int
	depth                int
	parens               int
	declarationHasEquals bool
	stack                []receiverEnclosure
}

// scanReceiverTrivia scans a compilation unit once. It retains only dots that
// can require receiver handling. Quoted text, comments and query text are not
// Apex receiver expressions.
func scanReceiverTrivia(source string) []receiverTrivia {
	if !receiverTriviaMayBePresent(source) {
		return nil
	}
	var tokens []receiverTriviaToken
	for i := 0; i < len(source); {
		start := i
		c := source[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f':
			i++
			continue
		case strings.HasPrefix(source[i:], "//"):
			if n := strings.IndexAny(source[i:], "\r\n"); n >= 0 {
				i += n
			} else {
				i = len(source)
			}
			continue
		case strings.HasPrefix(source[i:], "/*"):
			if n := strings.Index(source[i+2:], "*/"); n >= 0 {
				i += n + 4
			} else {
				i = len(source)
			}
			continue
		case strings.HasPrefix(source[i:], "'''"):
			i += 3
			if n := strings.Index(source[i:], "'''"); n >= 0 {
				i += n + 3
			} else {
				i = len(source)
			}
		case c == '\'':
			i++
			for i < len(source) {
				if source[i] == '\\' && i+1 < len(source) {
					i += 2
					continue
				}
				if source[i] == '\'' {
					i++
					break
				}
				i++
			}
		case c == '[' && receiverQueryStart(source[i+1:]):
			i++
			depth := 1
			for i < len(source) {
				if source[i] == '\'' {
					i++
					for i < len(source) {
						if source[i] == '\\' && i+1 < len(source) {
							i += 2
							continue
						}
						if source[i] == '\'' {
							i++
							break
						}
						i++
					}
					continue
				}
				if source[i] == '[' {
					depth++
				}
				if source[i] == ']' {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
		case isApexIdentifierChar(c) && !(c >= '0' && c <= '9'):
			i++
			for i < len(source) && isApexIdentifierChar(source[i]) {
				i++
			}
			word := source[start:i]
			tokens = append(tokens, receiverTriviaToken{text: word, start: start, end: i, word: !strings.EqualFold(word, "new") && (!isSemaKeyword(word) || strings.EqualFold(word, "this") || strings.EqualFold(word, "super") || strings.EqualFold(word, "trigger"))})
			continue
		case c >= '0' && c <= '9':
			i++
			for i < len(source) && (isApexIdentifierChar(source[i]) || source[i] == '.') {
				i++
			}
		default:
			i++
		}
		tokens = append(tokens, receiverTriviaToken{text: source[start:i], start: start, end: i})
	}
	var out []receiverTrivia
	var stack []receiverEnclosure
	statement, blockDepth, depth, parens := 0, 0, 0, 0
	declarationHasEquals := false
	for i, tok := range tokens {
		if tok.text == "\v" && i > 0 && i+1 < len(tokens) && (tokens[i-1].text == "." && tokens[i-1].end == tok.start || tokens[i+1].text == "." && tok.end == tokens[i+1].start) {
			out = append(out, receiverTrivia{kind: receiverInvalidControl, tokens: tokens, dot: i, statement: statement, blockDepth: blockDepth, depth: depth, parens: parens, declarationHasEquals: declarationHasEquals, stack: append([]receiverEnclosure(nil), stack...)})
		}
		if i > 0 && i+1 < len(tokens) && tok.text == "." && tokens[i-1].word && !(i >= 2 && tokens[i-2].text == "?" && tokens[i-2].end == tok.start) {
			var kind receiverTriviaKind
			if tok.start == tokens[i-1].end && tokens[i+1].start > tok.end {
				kind = receiverBoundBreak
			} else if tok.start > tokens[i-1].end {
				kind = receiverFreeName
			}
			if kind != 0 {
				out = append(out, receiverTrivia{kind: kind, tokens: tokens, dot: i, statement: statement, blockDepth: blockDepth, depth: depth, parens: parens, declarationHasEquals: declarationHasEquals, stack: append([]receiverEnclosure(nil), stack...)})
			}
		}
		switch tok.text {
		case "(":
			parens++
			kind := "group"
			if i > 0 {
				prev := strings.ToLower(tokens[i-1].text)
				switch prev {
				case "if", "while", "for", "switch":
					kind = prev
				default:
					if tokens[i-1].word || prev == ")" || prev == "]" {
						kind = "call"
					}
				}
				if i >= 2 && strings.EqualFold(tokens[i-2].text, "new") {
					kind = "new"
				}
			}
			stack = append(stack, receiverEnclosure{kind: kind, start: i})
		case "[":
			stack = append(stack, receiverEnclosure{kind: "index", start: i})
		case "{":
			depth++
			declarationHasEquals = false
			kind := "block"
			if i > 0 && (tokens[i-1].text == ">" || tokens[i-1].text == "]") {
				kind = "initializer"
			}
			if kind == "block" {
				blockDepth++
				statement = i + 1
			}
			stack = append(stack, receiverEnclosure{kind: kind, start: i})
		case ")", "]", "}":
			if tok.text == ")" {
				parens--
			}
			if tok.text == "}" {
				depth--
				declarationHasEquals = false
			}
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if tok.text == "}" && top.kind == "block" {
					blockDepth--
					statement = i + 1
				}
			}
		case ";":
			declarationHasEquals = false
			if len(stack) > 0 && stack[len(stack)-1].kind == "for" {
				stack[len(stack)-1].semicolons++
				continue
			}
			nested := false
			for _, frame := range stack {
				if frame.kind != "block" {
					nested = true
				}
			}
			if !nested {
				statement = i + 1
			}
		case "=":
			declarationHasEquals = true
		}
	}
	return out
}

// receiverTriviaMayBePresent cheaply rejects units with no dot adjacent to
// trivia. This byte check does not tokenize the unit or inspect receiver
// context; the lexer makes those decisions only when a candidate exists.
func receiverTriviaMayBePresent(source string) bool {
	triviaEdge := func(c byte) bool {
		return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f' || c == '\v' || c == '/'
	}
	for from := 0; from < len(source); {
		index := strings.IndexByte(source[from:], '.')
		if index < 0 {
			return false
		}
		index += from
		if index > 0 && triviaEdge(source[index-1]) || index+1 < len(source) && triviaEdge(source[index+1]) {
			return true
		}
		from = index + 1
	}
	return false
}

func receiverQueryStart(source string) bool {
	source = strings.TrimLeft(source, " \t\r\n\f")
	for _, keyword := range []string{"select", "find"} {
		if len(source) > len(keyword) && strings.EqualFold(source[:len(keyword)], keyword) && !isApexIdentifierChar(source[len(keyword)]) {
			return true
		}
	}
	return false
}
