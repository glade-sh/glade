package sema

import "strings"

type receiverTriviaKind uint8

const (
	receiverBoundBreak receiverTriviaKind = iota + 1
	receiverFreeName
)

type receiverTriviaToken struct {
	text       string
	start, end int
	word       bool
}

type receiverTrivia struct {
	kind   receiverTriviaKind
	tokens []receiverTriviaToken
	dot    int
}

// scanReceiverTrivia scans a compilation unit once. It retains only dots that
// can require receiver handling. Quoted text, comments and query text are not
// Apex receiver expressions.
func scanReceiverTrivia(source string) []receiverTrivia {
	var tokens []receiverTriviaToken
	for i := 0; i < len(source); {
		start := i
		c := source[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f':
			i++
			continue
		case strings.HasPrefix(source[i:], "//"):
			if n := strings.IndexByte(source[i:], '\n'); n >= 0 {
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
			tokens = append(tokens, receiverTriviaToken{text: word, start: start, end: i, word: !isSemaKeyword(word) || strings.EqualFold(word, "this") || strings.EqualFold(word, "super") || strings.EqualFold(word, "trigger")})
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
	for i := 1; i+1 < len(tokens); i++ {
		dot := tokens[i]
		if dot.text != "." || !tokens[i-1].word || (i >= 2 && tokens[i-2].text == "?" && tokens[i-2].end == dot.start) {
			continue
		}
		if dot.start == tokens[i-1].end && tokens[i+1].start > dot.end {
			out = append(out, receiverTrivia{kind: receiverBoundBreak, tokens: tokens, dot: i})
		} else if dot.start > tokens[i-1].end {
			out = append(out, receiverTrivia{kind: receiverFreeName, tokens: tokens, dot: i})
		}
	}
	return out
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
