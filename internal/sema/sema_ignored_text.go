package sema

import (
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// semaIgnoredText answers, for one body, whether an offset lies in a comment,
// a string literal or an inline SOQL/SOSL literal, and whether it lies inside
// an open parenthesis group. The body is scanned once, on the first query, and
// each query is a binary search. Callers that test many offsets of the same
// body must share one value; a fresh value per offset rescans the body.
type semaIgnoredText struct {
	body string

	spansReady bool
	ignored    []semaTextSpan // comments, strings and query literals; merged
	soql       []semaTextSpan // query literals only; merged

	parensReady bool
	parenAt     []int // offsets of counted '(' and ')'
	parenDepth  []int // depth after the event at the same index
}

// semaTextSpan is the half-open offset range [lo, hi).
type semaTextSpan struct {
	lo, hi int
}

const semaTextSpanOpen = math.MaxInt

func newSemaIgnoredText(body string) *semaIgnoredText {
	return &semaIgnoredText{body: body}
}

// contains reports whether pos lies in a comment, a string literal or an
// inline query literal of the body.
func (t *semaIgnoredText) contains(pos int) bool {
	t.buildSpans()
	return semaSpansContain(t.ignored, pos)
}

// inSOQLLiteral reports whether pos lies strictly inside the brackets of an
// inline SOQL or SOSL literal.
func (t *semaIgnoredText) inSOQLLiteral(pos int) bool {
	t.buildSpans()
	return semaSpansContain(t.soql, pos)
}

// inParenGroup reports whether an unclosed '(' outside comments and strings
// precedes pos.
func (t *semaIgnoredText) inParenGroup(pos int) bool {
	t.buildParens()
	n := sort.SearchInts(t.parenAt, pos)
	return n > 0 && t.parenDepth[n-1] > 0
}

func semaSpansContain(spans []semaTextSpan, pos int) bool {
	n := sort.Search(len(spans), func(i int) bool { return spans[i].lo > pos })
	return n > 0 && pos < spans[n-1].hi
}

func (t *semaIgnoredText) buildSpans() {
	if t.spansReady {
		return
	}
	t.spansReady = true
	t.soql = semaMergeSpans(semaSOQLLiteralSpans(t.body))
	t.ignored = semaMergeSpans(append(semaCommentAndStringSpans(t.body), t.soql...))
}

// semaCommentAndStringSpans records the offsets that the comment and string
// scan treats as ignored: a string covers the offsets after its opening quote
// through its closing quote, a line comment the offsets after its first '/'
// through the line break, and a block comment the offsets after its '/'
// through the '*' of its terminator. Unterminated comments run to the end.
func semaCommentAndStringSpans(body string) []semaTextSpan {
	var spans []semaTextSpan
	inBlock := false
	blockStart := 0
	for i := 0; i < len(body); i++ {
		if inBlock {
			if i+1 < len(body) && body[i] == '*' && body[i+1] == '/' {
				spans = append(spans, semaTextSpan{lo: blockStart + 1, hi: i + 1})
				inBlock = false
				i++
			}
			continue
		}
		if body[i] == '\'' {
			end := skipSemaString(body, i)
			spans = append(spans, semaTextSpan{lo: i + 1, hi: end + 1})
			i = end
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '*' {
			inBlock = true
			blockStart = i
			i++
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '/' {
			lineEnd := strings.IndexAny(body[i+2:], "\r\n")
			if lineEnd < 0 {
				return append(spans, semaTextSpan{lo: i + 1, hi: semaTextSpanOpen})
			}
			spans = append(spans, semaTextSpan{lo: i + 1, hi: i + 2 + lineEnd + 1})
			i += 2 + lineEnd
		}
	}
	if inBlock {
		spans = append(spans, semaTextSpan{lo: blockStart + 1, hi: semaTextSpanOpen})
	}
	return spans
}

// semaSOQLLiteralSpans records the offsets strictly between the brackets of
// each bracketed query that starts with SELECT or FIND. An unterminated query
// records nothing, and the scan resumes after its opening bracket.
func semaSOQLLiteralSpans(body string) []semaTextSpan {
	var spans []semaTextSpan
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(body, i)
		case '[':
			queryStart := i + 1
			for queryStart < len(body) && isWhitespace(body[queryStart]) {
				queryStart++
			}
			if !semaHasLowerPrefix(body[queryStart:], "select") && !semaHasLowerPrefix(body[queryStart:], "find") {
				continue
			}
			depth := 1
			for j := i + 1; j < len(body); j++ {
				switch body[j] {
				case '/':
					if end, ok := skipSemaComment(body, j); ok {
						j = end
					}
				case '\'':
					j = skipSemaString(body, j)
				case '[':
					depth++
				case ']':
					depth--
					if depth == 0 {
						spans = append(spans, semaTextSpan{lo: i + 1, hi: j})
						i = j
						j = len(body)
					}
				}
			}
		}
	}
	return spans
}

// semaHasLowerPrefix reports strings.HasPrefix(strings.ToLower(s), prefix) for
// an ASCII prefix without lowering the rest of s. strings.ToLower maps whole
// runes, so a non-ASCII rune that lowers to an ASCII letter still matches, and
// an invalid byte never does.
func semaHasLowerPrefix(s, prefix string) bool {
	for k := 0; k < len(prefix); k++ {
		r, size := utf8.DecodeRuneInString(s)
		if (r == utf8.RuneError && size <= 1) || unicode.ToLower(r) != rune(prefix[k]) {
			return false
		}
		s = s[size:]
	}
	return true
}

// semaMergeSpans sorts spans and merges overlapping or touching ones.
func semaMergeSpans(spans []semaTextSpan) []semaTextSpan {
	out := spans[:0]
	for _, span := range spans {
		if span.lo < span.hi {
			out = append(out, span)
		}
	}
	if len(out) < 2 {
		return out
	}
	sort.Slice(out, func(i, j int) bool { return out[i].lo < out[j].lo })
	merged := out[:1]
	for _, span := range out[1:] {
		last := &merged[len(merged)-1]
		if span.lo <= last.hi {
			last.hi = max(last.hi, span.hi)
			continue
		}
		merged = append(merged, span)
	}
	return merged
}

// buildParens records the depth changes of the parenthesis scan. Strings and
// comments are skipped; a line comment runs to '\n' only.
func (t *semaIgnoredText) buildParens() {
	if t.parensReady {
		return
	}
	t.parensReady = true
	body := t.body
	depth := 0
	inBlock := false
	for i := 0; i < len(body); i++ {
		if inBlock {
			if i+1 < len(body) && body[i] == '*' && body[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if body[i] == '\'' {
			i = skipSemaString(body, i)
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '*' {
			inBlock = true
			i++
			continue
		}
		if i+1 < len(body) && body[i] == '/' && body[i+1] == '/' {
			for i < len(body) && body[i] != '\n' {
				i++
			}
			continue
		}
		switch body[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				continue
			}
			depth--
		default:
			continue
		}
		t.parenAt = append(t.parenAt, i)
		t.parenDepth = append(t.parenDepth, depth)
	}
}
