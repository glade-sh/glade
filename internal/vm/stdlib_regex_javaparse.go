package vm

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf16"
)

// javaRegexSyntaxCheck walks a pattern with the cursor discipline of
// java.util.regex.Pattern and reports the first syntax error Java reports,
// with its description and index. It validates only; the translator in
// stdlib_regex_engine.go still produces the executable pattern. The indexes
// and texts are recorded by the regex_java_divergence native rows.
type javaSyntaxError struct {
	description string
	index       int
}

type javaTreeInfo struct {
	minLen, maxLen int32
	maxValid       bool
	deterministic  bool
	unbounded      bool // holds an unbounded repetition
}

func javaFixedInfo(n int32) javaTreeInfo {
	return javaTreeInfo{minLen: n, maxLen: n, maxValid: true, deterministic: true}
}

type javaRegexParser struct {
	t         []rune
	n         int
	cursor    int
	comments  bool
	unixLines bool
	skipped   []bool // characters a (?x) read skipped, by index in t
	sawX      bool
	groups    int
	names     map[string]bool

	escapeInfo javaTreeInfo

	loops    []javaLoopGroup
	nested   bool            // an unbounded repeat of a group that holds an unbounded repeat
	repeated []javaLoopGroup // every group followed by a repeating quantifier
}

// javaLoopGroup is a quantified group whose body Java cannot match in a fixed
// way. Java matches it with a recursive Loop node, one stack frame set per
// iteration. start is the index of the opening parenthesis and end the index
// after the closing one, in the \Q-normalised pattern.
type javaLoopGroup struct {
	start, end int
}

const (
	javaMaxReps = math.MaxInt32
)

func javaRegexSyntaxCheck(pattern string) (err *javaSyntaxError) {
	_, err = javaRegexParse(pattern)
	return err
}

// javaRegexLoopGroups returns the groups Java matches with recursive loops.
func javaRegexLoopGroups(normalized string) []javaLoopGroup {
	p, err := javaRegexParse(normalized)
	if err != nil || p == nil {
		return nil
	}
	// Only outermost loops: an inner loop's captures accumulate across the outer
	// iterations, which do not nest on the stack.
	var outer []javaLoopGroup
	for _, loop := range p.loops {
		nested := false
		for _, other := range p.repeated {
			if other != loop && other.start <= loop.start && other.end >= loop.end {
				nested = true
				break
			}
		}
		if !nested {
			outer = append(outer, loop)
		}
	}
	return outer
}

func javaRegexParse(pattern string) (parser *javaRegexParser, err *javaSyntaxError) {
	p := &javaRegexParser{names: map[string]bool{}, groups: 1}
	p.t = javaRemoveQEQuoting([]rune(pattern))
	p.n = len(p.t)
	p.t = append(p.t, 0, 0)
	defer func() {
		if r := recover(); r != nil {
			if se, ok := r.(*javaSyntaxError); ok {
				parser, err = nil, se
				return
			}
			panic(r)
		}
	}()
	p.expr()
	if p.cursor != p.n {
		if p.peek() == ')' {
			p.fail("Unmatched closing ')'")
		}
		p.fail("Unexpected character '" + string(p.peek()) + "'")
	}
	return p, nil
}

// javaRemoveQEQuoting mirrors Pattern.RemoveQEQuoting, which rewrites \Q...\E
// before parsing and so shifts every later index.
func javaRemoveQEQuoting(temp []rune) []rune {
	pLen := len(temp)
	i := 0
	for i < pLen-1 {
		if temp[i] != '\\' {
			i++
		} else if temp[i+1] != 'Q' {
			i += 2
		} else {
			break
		}
	}
	if i >= pLen-1 {
		return temp
	}
	j := i
	i += 2
	out := append([]rune(nil), temp[:j]...)
	inQuote, beginQuote := true, true
	isAlpha := func(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	isDigit := func(c rune) bool { return c >= '0' && c <= '9' }
	at := func(k int) rune {
		if k < pLen {
			return temp[k]
		}
		return 0
	}
	for i < pLen {
		c := temp[i]
		i++
		switch {
		case c >= 128 || isAlpha(c):
			out = append(out, c)
		case isDigit(c):
			if beginQuote {
				out = append(out, '\\', 'x', '3')
			}
			out = append(out, c)
		case c != '\\':
			if inQuote {
				out = append(out, '\\')
			}
			out = append(out, c)
		case inQuote:
			if at(i) == 'E' {
				i++
				inQuote = false
			} else {
				out = append(out, '\\', '\\')
			}
		default:
			if at(i) == 'Q' {
				i++
				inQuote = true
				beginQuote = true
				continue
			}
			out = append(out, c)
			if i != pLen {
				out = append(out, temp[i])
				i++
			}
		}
		beginQuote = false
	}
	return out
}

func (p *javaRegexParser) fail(description string) {
	panic(&javaSyntaxError{description: description, index: p.cursor - 1})
}

func (p *javaRegexParser) at(i int) rune {
	if i < 0 || i >= len(p.t) {
		return 0
	}
	return p.t[i]
}

func javaASCIISpace(c rune) bool { return c == ' ' || c >= 9 && c <= 13 }

func (p *javaRegexParser) lineSeparator(ch rune) bool {
	if p.unixLines {
		return ch == '\n'
	}
	return ch == '\n' || ch == '\r' || ch == 0x85 || ch == 0x2028 || ch == 0x2029
}

// skip records that the character at i is whitespace or comment text that a
// (?x) read stepped over; it is not part of the pattern Java compiles.
func (p *javaRegexParser) skipChar(i int) {
	if i >= 0 && i < p.n {
		if p.skipped == nil {
			p.skipped = make([]bool, p.n)
		}
		p.skipped[i] = true
	}
}

// keepRaw clears the skip marks over t[from:to]: Java reads a \N{name} or
// \p{name} body raw, so its spaces belong to the pattern.
func (p *javaRegexParser) keepRaw(from, to int) {
	for i := from; i < to && i < len(p.skipped); i++ {
		if i >= 0 {
			p.skipped[i] = false
		}
	}
}

// pastWhitespace is Java's peekPastWhitespace: the cursor ends on the first
// character that is neither whitespace nor part of a comment, and ch is it.
func (p *javaRegexParser) pastWhitespace(ch rune) rune {
	for javaASCIISpace(ch) || ch == '#' {
		for javaASCIISpace(ch) {
			p.skipChar(p.cursor)
			p.cursor++
			ch = p.at(p.cursor)
		}
		if ch == '#' {
			p.skipChar(p.cursor)
			p.cursor++
			ch = p.at(p.cursor)
			for ch != 0 && !p.lineSeparator(ch) {
				p.skipChar(p.cursor)
				p.cursor++
				ch = p.at(p.cursor)
			}
		}
	}
	return ch
}

// parsePastWhitespace is Java's parsePastWhitespace: ch was just consumed, and
// the cursor ends after the character it returns.
func (p *javaRegexParser) parsePastWhitespace(ch rune) rune {
	for javaASCIISpace(ch) || ch == '#' {
		for javaASCIISpace(ch) {
			p.skipChar(p.cursor - 1)
			ch = p.at(p.cursor)
			p.cursor++
		}
		if ch == '#' {
			p.skipChar(p.cursor - 1)
			ch = p.at(p.cursor)
			p.cursor++
			for ch != 0 && !p.lineSeparator(ch) {
				p.skipChar(p.cursor - 1)
				ch = p.at(p.cursor)
				p.cursor++
			}
		}
	}
	return ch
}

func (p *javaRegexParser) peek() rune {
	ch := p.at(p.cursor)
	if p.comments {
		ch = p.pastWhitespace(ch)
	}
	return ch
}

func (p *javaRegexParser) read() rune {
	ch := p.at(p.cursor)
	p.cursor++
	if p.comments {
		ch = p.parsePastWhitespace(ch)
	}
	return ch
}

func (p *javaRegexParser) next() rune {
	p.cursor++
	ch := p.at(p.cursor)
	if p.comments {
		ch = p.pastWhitespace(ch)
	}
	return ch
}

func (p *javaRegexParser) nextEscaped() rune {
	p.cursor++
	return p.at(p.cursor)
}

func (p *javaRegexParser) unread() { p.cursor-- }

func (p *javaRegexParser) skip() rune {
	i := p.cursor
	ch := p.at(i + 1)
	p.cursor = i + 2
	return ch
}

func (p *javaRegexParser) accept(ch rune, description string) {
	test := p.at(p.cursor)
	p.cursor++
	if p.comments {
		test = p.parsePastWhitespace(test)
	}
	if test != ch {
		p.fail(description)
	}
}

func javaInfoAppend(a, b javaTreeInfo) javaTreeInfo {
	out := javaTreeInfo{minLen: a.minLen + b.minLen, maxLen: a.maxLen + b.maxLen, unbounded: a.unbounded || b.unbounded}
	out.maxValid = a.maxValid && b.maxValid
	out.deterministic = a.deterministic && b.deterministic
	return out
}

func (p *javaRegexParser) expr() javaTreeInfo {
	var branches []javaTreeInfo
	for {
		branches = append(branches, p.sequence())
		if p.peek() != '|' {
			break
		}
		p.next()
	}
	if len(branches) == 1 {
		return branches[0]
	}
	out := javaTreeInfo{minLen: math.MaxInt32, maxLen: -1, maxValid: true}
	for _, b := range branches {
		if b.minLen < out.minLen {
			out.minLen = b.minLen
		}
		if b.maxLen > out.maxLen {
			out.maxLen = b.maxLen
		}
		out.maxValid = out.maxValid && b.maxValid
		out.unbounded = out.unbounded || b.unbounded
	}
	out.deterministic = false
	return out
}

func (p *javaRegexParser) sequence() javaTreeInfo {
	info := javaFixedInfo(0)
	for {
		ch := p.peek()
		var node javaTreeInfo
		switch ch {
		case '(':
			node = p.group0()
			if node.minLen == -1 {
				// inline modifier group: no node
				continue
			}
			info = javaInfoAppend(info, node)
			continue
		case '[':
			p.clazz(true)
			node = javaFixedInfo(1)
		case '\\':
			ch = p.nextEscaped()
			if ch == 'p' || ch == 'P' {
				oneLetter := true
				ch = p.next()
				if ch != '{' {
					p.unread()
				} else {
					oneLetter = false
				}
				p.family(oneLetter)
				node = javaFixedInfo(1)
			} else {
				p.unread()
				node = p.atom()
			}
		case '^':
			p.next()
			node = javaFixedInfo(0)
		case '$':
			p.next()
			node = javaFixedInfo(0)
		case '.':
			p.next()
			node = javaFixedInfo(1)
		case '|', ')':
			return info
		case ']', '}':
			node = p.atom()
		case '?', '*', '+':
			p.next()
			p.fail("Dangling meta character '" + string(ch) + "'")
		case 0:
			if p.cursor >= p.n {
				return info
			}
			node = p.atom()
		default:
			node = p.atom()
		}
		node = p.closure(node)
		info = javaInfoAppend(info, node)
	}
}

type javaQuant struct {
	set        bool
	cmin, cmax int64
	possessive bool
	question   bool
}

// closure applies a trailing quantifier to a node that is not a group.
func (p *javaRegexParser) closure(prev javaTreeInfo) javaTreeInfo {
	q := p.quantifier()
	if !q.set {
		return prev
	}
	return javaCurlyInfo(prev, q)
}

func javaCurlyInfo(atom javaTreeInfo, q javaQuant) javaTreeInfo {
	if q.question {
		return javaTreeInfo{minLen: 0, maxLen: atom.maxLen, maxValid: atom.maxValid, deterministic: false, unbounded: atom.unbounded}
	}
	out := javaTreeInfo{maxValid: atom.maxValid, deterministic: atom.deterministic && q.cmin == q.cmax, unbounded: atom.unbounded || q.cmax == javaMaxReps}
	out.minLen = int32(int64(atom.minLen) * q.cmin)
	if int64(atom.minLen)*q.cmin > math.MaxInt32 || int64(out.minLen) < 0 {
		out.minLen = 0xFFFFFFF
	}
	max := int64(int32(int64(atom.maxLen) * q.cmax))
	out.maxLen = int32(max)
	if int32(max) < atom.maxLen {
		out.maxValid = false
	}
	return out
}

// quantifier reads ?, *, +, or {n,m} with an optional lazy or possessive suffix.
func (p *javaRegexParser) quantifier() javaQuant {
	ch := p.peek()
	suffix := func(q javaQuant) javaQuant {
		next := p.next()
		if next == '?' {
			p.next()
		} else if next == '+' {
			q.possessive = true
			p.next()
		}
		return q
	}
	switch ch {
	case '?':
		q := suffix(javaQuant{set: true, cmin: 0, cmax: 1, question: true})
		return q
	case '*':
		return suffix(javaQuant{set: true, cmin: 0, cmax: javaMaxReps})
	case '+':
		return suffix(javaQuant{set: true, cmin: 1, cmax: javaMaxReps})
	case '{':
		first := p.at(p.cursor + 1)
		if first < '0' || first > '9' {
			p.cursor += 2
			p.fail("Illegal repetition")
		}
		p.cursor += 2
		var cmin, cmax int64
		overflow := func(v int64) bool { return v > math.MaxInt32 }
		ch = first
		for {
			cmin = cmin*10 + int64(ch-'0')
			if overflow(cmin) {
				p.fail("Illegal repetition range")
			}
			ch = p.read()
			if ch < '0' || ch > '9' {
				break
			}
		}
		cmax = cmin
		if ch == ',' {
			ch = p.read()
			cmax = javaMaxReps
			if ch != '}' {
				cmax = 0
				for ch >= '0' && ch <= '9' {
					cmax = cmax*10 + int64(ch-'0')
					if overflow(cmax) {
						p.fail("Illegal repetition range")
					}
					ch = p.read()
				}
			}
		}
		if ch != '}' {
			p.fail("Unclosed counted closure")
		}
		if cmax < cmin {
			p.fail("Illegal repetition range")
		}
		q := javaQuant{set: true, cmin: cmin, cmax: cmax}
		ch = p.peek()
		if ch == '?' {
			p.next()
		} else if ch == '+' {
			p.next()
			q.possessive = true
		}
		return q
	}
	return javaQuant{}
}

func (p *javaRegexParser) atom() javaTreeInfo {
	first := 0
	prev := -1
	count := int32(0)
	ch := p.peek()
	for {
		stop := false
		switch ch {
		case '*', '+', '?', '{':
			if first > 1 {
				p.cursor = prev
				first--
				count--
			}
			stop = true
		case '$', '.', '^', '(', '[', '|', ')':
			stop = true
		case '\\':
			ch = p.nextEscaped()
			if ch == 'p' || ch == 'P' {
				if first > 0 {
					// A literal run is waiting: leave the cursor on the backslash.
					p.unread()
					stop = true
					break
				}
				oneLetter := true
				ch = p.next()
				if ch != '{' {
					p.unread()
				} else {
					oneLetter = false
				}
				p.family(oneLetter)
				return javaFixedInfo(1)
			}
			p.unread()
			prev = p.cursor
			c, kind := p.escape(false, first == 0, false)
			if kind == javaEscapeChar {
				first++
				count++
				ch = p.peek()
				_ = c
				continue
			} else if first == 0 {
				return p.escapeInfo
			}
			p.cursor = prev
			stop = true
		case 0:
			if p.cursor >= p.n {
				stop = true
				break
			}
			fallthrough
		default:
			prev = p.cursor
			first++
			count++
			ch = p.next()
			continue
		}
		if stop {
			break
		}
	}
	return javaFixedInfo(count)
}

type javaEscapeKind int

const (
	javaEscapeChar javaEscapeKind = iota
	javaEscapeClass
	javaEscapeZero
	javaEscapeBackref
)

// escape parses one backslash escape. The cursor starts at the backslash.
func (p *javaRegexParser) escape(inclass, create, isrange bool) (rune, javaEscapeKind) {
	ch := p.skip()
	p.escapeInfo = javaFixedInfo(1)
	switch ch {
	case '0':
		return p.octal(), javaEscapeChar
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if inclass {
			break
		}
		p.backref(int(ch - '0'))
		p.escapeInfo = javaTreeInfo{maxValid: false, deterministic: true}
		return 0, javaEscapeBackref
	case 'A', 'G', 'Z', 'z':
		if inclass {
			break
		}
		p.escapeInfo = javaFixedInfo(0)
		return 0, javaEscapeZero
	case 'B':
		if inclass {
			break
		}
		p.escapeInfo = javaFixedInfo(0)
		return 0, javaEscapeZero
	case 'D', 'H', 'S', 'V', 'W', 'd', 'h', 's', 'w':
		return 0, javaEscapeClass
	case 'v':
		return 0, javaEscapeClass
	case 'N':
		return p.namedChar(), javaEscapeChar
	case 'R', 'X':
		if inclass {
			break
		}
		return 0, javaEscapeClass
	case 'a':
		return 7, javaEscapeChar
	case 'b':
		if inclass {
			break
		}
		if p.peek() == '{' {
			if p.skip() == 'g' {
				if p.read() == '}' {
					p.escapeInfo = javaFixedInfo(0)
					return 0, javaEscapeZero
				}
				break
			}
			p.unread()
			p.unread()
		}
		p.escapeInfo = javaFixedInfo(0)
		return 0, javaEscapeZero
	case 'c':
		if p.cursor < p.n {
			return p.read() ^ 64, javaEscapeChar
		}
		p.fail("Illegal control escape sequence")
	case 'e':
		return 0x1B, javaEscapeChar
	case 'f':
		return '\f', javaEscapeChar
	case 'k':
		if inclass {
			break
		}
		if p.read() != '<' {
			p.fail("\\k is not followed by '<' for named capturing group")
		}
		name := p.groupName(p.read())
		if !p.names[name] {
			p.fail("named capturing group <" + name + "> does not exist")
		}
		p.escapeInfo = javaTreeInfo{maxValid: false, deterministic: true}
		return 0, javaEscapeBackref
	case 'n':
		return '\n', javaEscapeChar
	case 'r':
		return '\r', javaEscapeChar
	case 't':
		return '\t', javaEscapeChar
	case 'u':
		return p.unicodeEscape(), javaEscapeChar
	case 'x':
		return p.hexEscape(), javaEscapeChar
	default:
		if ch == 0 && p.cursor > p.n {
			p.fail("Unescaped trailing backslash")
		}
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' {
			break
		}
		return ch, javaEscapeChar
	}
	p.fail("Illegal/unsupported escape sequence")
	return 0, javaEscapeChar
}

func (p *javaRegexParser) backref(first int) {
	refNum := first
	for {
		ch := p.peek()
		if ch < '0' || ch > '9' {
			return
		}
		candidate := refNum*10 + int(ch-'0')
		if p.groups-1 < candidate {
			return
		}
		refNum = candidate
		p.read()
	}
}

func isOctal(n rune) bool { return n >= '0' && n <= '7' }

func (p *javaRegexParser) octal() rune {
	n := p.read()
	if isOctal(n) {
		m := p.read()
		if isOctal(m) {
			o := p.read()
			if isOctal(o) && n <= '3' {
				return (n-'0')*64 + (m-'0')*8 + (o - '0')
			}
			p.unread()
			return (n-'0')*8 + (m - '0')
		}
		p.unread()
		return n - '0'
	}
	p.fail("Illegal octal escape sequence")
	return 0
}

func isHexRune(c rune) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func hexRuneValue(c rune) rune {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	}
	return c - 'A' + 10
}

func (p *javaRegexParser) hexEscape() rune {
	n := p.read()
	if isHexRune(n) {
		m := p.read()
		if isHexRune(m) {
			return hexRuneValue(n)*16 + hexRuneValue(m)
		}
	} else if n == '{' && isHexRune(p.peek()) {
		ch := rune(0)
		for {
			n = p.read()
			if !isHexRune(n) {
				break
			}
			ch = ch<<4 + hexRuneValue(n)
			if ch > 0x10FFFF {
				p.fail("Hexadecimal codepoint is too big")
			}
		}
		if n != '}' {
			p.fail("Unclosed hexadecimal escape sequence")
		}
		return ch
	}
	p.fail("Illegal hexadecimal escape sequence")
	return 0
}

func (p *javaRegexParser) uxxxx() rune {
	n := rune(0)
	for i := 0; i < 4; i++ {
		ch := p.read()
		if !isHexRune(ch) {
			p.fail("Illegal Unicode escape sequence")
		}
		n = n*16 + hexRuneValue(ch)
	}
	return n
}

func (p *javaRegexParser) unicodeEscape() rune {
	n := p.uxxxx()
	if utf16.IsSurrogate(n) && n < 0xDC00 {
		save := p.cursor
		if p.read() == '\\' && p.read() == 'u' {
			lo := p.uxxxx()
			if lo >= 0xDC00 && lo <= 0xDFFF {
				return utf16.DecodeRune(n, lo)
			}
		}
		p.cursor = save
	}
	return n
}

func (p *javaRegexParser) namedChar() rune {
	if p.read() == '{' {
		start := p.cursor
		for p.read() != '}' {
			if p.cursor >= p.n {
				p.fail("Unclosed character name escape sequence")
			}
		}
		p.keepRaw(start, p.cursor-1)
		name := string(p.t[start : p.cursor-1])
		if r, ok := javaCharacterByName(name); ok {
			return r
		}
		p.fail("Unknown character name [" + name + "]")
	}
	p.fail("Illegal character name escape sequence")
	return 0
}

func (p *javaRegexParser) groupName(ch rune) string {
	isAlpha := func(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	isAlnum := func(c rune) bool { return isAlpha(c) || c >= '0' && c <= '9' }
	if !isAlpha(ch) {
		p.fail("capturing group name does not start with a Latin letter")
	}
	var sb strings.Builder
	for {
		sb.WriteRune(ch)
		ch = p.read()
		if !isAlnum(ch) {
			break
		}
	}
	if ch != '>' {
		p.fail("named capturing group is missing trailing '>'")
	}
	return sb.String()
}

// family validates \p{Name}, \P{Name}, \pX and \PX.
func (p *javaRegexParser) family(singleLetter bool) {
	p.next()
	var name string
	if singleLetter {
		// Java names the property after whatever is there, including the NUL past
		// the end of the pattern (regex_java_divergence d1_x_bare).
		name = string(p.at(p.cursor))
		p.read()
	} else {
		i := p.cursor
		p.t[p.n] = '}'
		for p.read() != '}' {
		}
		p.t[p.n] = 0
		j := p.cursor
		if j > p.n {
			p.fail("Unclosed character family")
		}
		if i+1 >= j {
			p.fail("Empty character family")
		}
		p.keepRaw(i, j-1)
		name = string(p.t[i : j-1])
	}
	if javaNativeCategoryNames[name] {
		return
	}
	if _, perr := javaPropertySet(name, false, false); perr != nil {
		p.fail(perr.description)
	}
}

func (p *javaRegexParser) addFlag() {
	ch := p.peek()
	for {
		switch ch {
		case 'i', 'm', 's', 'u', 'c', 'U':
		case 'd':
			p.unixLines = true
		case 'x':
			p.comments = true
			p.sawX = true
		case '-':
			ch = p.next()
			for {
				switch ch {
				case 'i', 'm', 's', 'u', 'c', 'U':
				case 'd':
					p.unixLines = false
				case 'x':
					p.comments = false
				default:
					return
				}
				ch = p.next()
			}
		default:
			return
		}
		ch = p.next()
	}
}

// group0 parses a group and its quantifier. A flag-only group "(?i)" returns
// minLen -1 because it adds no node.
func (p *javaRegexParser) group0() javaTreeInfo {
	groupStart := p.cursor
	saveComments, saveUnix := p.comments, p.unixLines
	var body javaTreeInfo
	zeroWidth := false
	ch := p.next()
	if ch == '?' {
		ch = p.skip()
		switch ch {
		case ':', '>':
			body = p.expr()
		case '=', '!':
			body = p.expr()
			zeroWidth = true
		case '<':
			ch = p.read()
			if ch != '=' && ch != '!' {
				name := p.groupName(ch)
				if p.names[name] {
					p.fail("Named capturing group <" + name + "> is already defined")
				}
				p.names[name] = true
				p.groups++
				body = p.expr()
			} else {
				body = p.expr()
				if !body.maxValid {
					p.fail("Look-behind group does not have an obvious maximum length")
				}
				zeroWidth = true
			}
		case '$', '@':
			p.fail("Unknown group type")
		default:
			p.unread()
			p.addFlag()
			ch = p.read()
			if ch == ')' {
				return javaTreeInfo{minLen: -1}
			}
			if ch != ':' {
				p.fail("Unknown inline modifier")
			}
			body = p.expr()
		}
	} else {
		p.groups++
		body = p.expr()
	}
	p.accept(')', "Unclosed group")
	p.comments, p.unixLines = saveComments, saveUnix
	groupEnd := p.cursor
	q := p.quantifier()
	if zeroWidth {
		return javaFixedInfo(0)
	}
	if !q.set {
		return body
	}
	if !q.question {
		p.repeated = append(p.repeated, javaLoopGroup{start: groupStart, end: groupEnd})
		if q.cmax == javaMaxReps && body.unbounded {
			p.nested = true
		}
	}
	if q.possessive || q.question || body.deterministic {
		return javaCurlyInfo(body, q)
	}
	p.loops = append(p.loops, javaLoopGroup{start: groupStart, end: groupEnd})
	return javaTreeInfo{minLen: 0, maxLen: 0, maxValid: false, deterministic: false, unbounded: body.unbounded || q.cmax == javaMaxReps}
}

// clazz parses a character class; the cursor starts at the opening bracket.
func (p *javaRegexParser) clazz(consume bool) {
	have := false
	ch := p.next()
	if ch == '^' && p.at(p.cursor-1) == '[' {
		ch = p.next()
	}
	for {
		switch ch {
		case '[':
			p.clazz(true)
			have = true
			ch = p.peek()
			continue
		case '&':
			ch = p.next()
			if ch == '&' {
				ch = p.next()
				right := false
				for ch != ']' && ch != '&' {
					if ch == '[' {
						p.clazz(true)
					} else {
						p.unread()
						p.clazz(false)
					}
					right = true
					ch = p.peek()
				}
				if !have && !right {
					p.fail("Bad class syntax")
				}
				have = true
				continue
			}
			p.unread()
		case 0:
			if p.cursor >= p.n {
				p.fail("Unclosed character class")
			}
		case ']':
			if have {
				if consume {
					p.next()
				}
				return
			}
		}
		p.rangeItem()
		have = true
		ch = p.peek()
	}
}

func (p *javaRegexParser) rangeItem() {
	ch := p.peek()
	var c rune
	if ch == '\\' {
		ch = p.nextEscaped()
		if ch == 'p' || ch == 'P' {
			oneLetter := true
			ch = p.next()
			if ch != '{' {
				p.unread()
			} else {
				oneLetter = false
			}
			p.family(oneLetter)
			return
		}
		p.unread()
		var kind javaEscapeKind
		c, kind = p.escape(true, true, false)
		if kind != javaEscapeChar {
			return
		}
	} else {
		c = ch
		p.next()
	}
	if p.peek() == '-' {
		endRange := p.at(p.cursor + 1)
		if endRange == '[' {
			return
		}
		if endRange != ']' {
			p.next()
			m := p.peek()
			mIsChar := true
			if m == '\\' {
				var kind javaEscapeKind
				m, kind = p.escape(true, false, true)
				mIsChar = kind == javaEscapeChar
			} else {
				p.next()
			}
			if !mIsChar || m < c {
				p.fail("Illegal character range")
			}
		}
	}
}

// javaRegexNestsUnboundedRepeats reports an unbounded repeat of a group that
// itself holds an unbounded repeat, such as (a+)+. Backtracking engines take
// exponential time to fail on these.
func javaRegexNestsUnboundedRepeats(normalized string) bool {
	p, err := javaRegexParse(normalized)
	return err == nil && p != nil && p.nested
}

// javaRegexExtendedNormalize returns the pattern Java compiles from a
// COMMENTS-mode pattern: the characters its reads kept, with the x flag
// removed. It follows Java's own cursor, so it is total for valid patterns.
// ok is false when x is never enabled or the pattern is invalid.
func javaRegexExtendedNormalize(pattern string) (string, bool) {
	p, err := javaRegexParse(pattern)
	if err != nil || p == nil || !p.sawX {
		return pattern, false
	}
	var kept strings.Builder
	for i, r := range p.t[:p.n] {
		if p.skipped != nil && p.skipped[i] {
			continue
		}
		switch r {
		case 0:
			kept.WriteString(`\x00`)
		case 0x85, 0x2028, 0x2029:
			kept.WriteString(fmt.Sprintf(`\x{%X}`, r))
		default:
			kept.WriteRune(r)
		}
	}
	out := javaDropExtendedFlag(kept.String())
	if javaRegexSyntaxCheck(out) != nil {
		return pattern, false
	}
	return out, true
}

// javaDropExtendedFlag removes x from the enabling part of every flag group:
// (?x) goes, (?ix) becomes (?i) and (?x: becomes (?: . (?-x) is left alone.
func javaDropExtendedFlag(src string) string {
	var out strings.Builder
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\' && i+1 < len(src):
			out.WriteByte(c)
			i++
			out.WriteByte(src[i])
		case c == '[':
			end := javaClassEndIndex(src, i)
			out.WriteString(src[i : end+1])
			i = end
		case c == '(' && i+1 < len(src) && src[i+1] == '?':
			j := i + 2
			for j < len(src) && strings.IndexByte("idmsuxU-", src[j]) >= 0 {
				j++
			}
			if j < len(src) && (src[j] == ')' || src[j] == ':') {
				flags := src[i+2 : j]
				on, off, hasOff := flags, "", false
				if k := strings.IndexByte(flags, '-'); k >= 0 {
					on, off, hasOff = flags[:k], flags[k:], true
				}
				_ = hasOff
				on = strings.ReplaceAll(on, "x", "")
				if on == "" && off == "" && src[j] == ')' {
					i = j
					continue
				}
				out.WriteString("(?" + on + off + string(src[j]))
				i = j
				continue
			}
			out.WriteString("(?")
			i++
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// javaClassEndIndex returns the index of the ] that closes the class opened at
// start (nested classes count; a ] right after [ or [^ is a literal), or the
// last index when the class never closes.
func javaClassEndIndex(src string, start int) int {
	depth := 1
	i := start + 1
	if i < len(src) && src[i] == '^' {
		i++
	}
	if i < len(src) && src[i] == ']' {
		i++
	}
	for ; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(src) - 1
}
