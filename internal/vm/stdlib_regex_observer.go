package vm

import (
	"fmt"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// The observation expression follows the engine's ordered alternatives and
// quantifiers. A read attempted at EOF records a private capture and finishes
// that path. It never supplies the result or public captures of the real match.
// This includes reads on abandoned alternatives (R049-R051), greedy repetition
// (R019-R048), and failed possessive matching (R031/R033).
func regexp2EndObserverSource(source string, re *regexp2.Regexp) (string, int) {
	slot := 1
	for _, number := range re.GetGroupNumbers() {
		if number >= slot {
			slot = number + 1
		}
	}
	// Keep numeric escapes from acquiring a new backreference interpretation.
	for strings.Contains(source, `\`+strconv.Itoa(slot)) {
		slot++
	}
	name := strconv.Itoa(slot)
	mark := "(?<" + name + `>\z)`
	wrap := func(atom string, readsEnd bool) string {
		if readsEnd {
			return "(?(" + name + ")|(?:" + atom + "|" + mark + "))"
		}
		return "(?(" + name + ")|" + atom + ")"
	}
	closerAt := map[int]string{}
	var out strings.Builder
	// Anchor-only expressions still need a declared slot for the conditions.
	// This definition cannot capture or change the observation result.
	out.WriteString("(?:(?<" + name + ">(?!))|)")
	extended := false
	var stack []bool
	for i := 0; i < len(source); {
		start, ch := i, source[i]
		if extended && ch == '#' {
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				out.WriteString(source[i:])
				break
			}
			out.WriteString(source[i : i+end+1])
			i += end + 1
			continue
		}
		if extended && strings.ContainsRune(" \t\r\n\f", rune(ch)) {
			out.WriteByte(ch)
			i++
			continue
		}
		switch ch {
		case '(':
			if strings.HasPrefix(source[i:], "(?#") {
				if end := strings.IndexByte(source[i+3:], ')'); end >= 0 {
					i += end + 4
					out.WriteString(source[start:i])
					continue
				}
			}
			if strings.HasPrefix(source[i:], "(?!") {
				// A negative lookahead drops captures made inside it, which would
				// hide reads at the end of the input. Observe its body through a
				// positive lookahead that the path may skip.
				out.WriteString("(?:(?=")
				i += 3
				stack = append(stack, extended)
				closerAt[len(stack)] = ")|)"
				continue
			}
			if strings.HasPrefix(source[i:], "(?") {
				j, scoped, enabled, sawFlag := i+2, extended, true, false
				for j < len(source) && strings.ContainsRune("imnsxIMNSX-+", rune(source[j])) {
					if source[j] == '-' {
						enabled = false
					} else if source[j] == '+' {
						enabled = true
					} else {
						sawFlag = true
						if source[j] == 'x' || source[j] == 'X' {
							scoped = enabled
						}
					}
					j++
				}
				if sawFlag && j < len(source) && (source[j] == ':' || source[j] == ')') {
					if source[j] == ':' {
						stack = append(stack, extended)
					}
					extended = scoped
					i = j + 1
					out.WriteString(source[start:i])
					continue
				}
				if i+3 < len(source) && source[i+2] == '<' && source[i+3] != '=' && source[i+3] != '!' {
					if end := strings.IndexByte(source[i+3:], '>'); end >= 0 {
						i += end + 4
					} else {
						i += 2
					}
				} else if i+3 < len(source) && source[i+2] == '<' {
					i += 4
				} else {
					i += 3
				}
			} else {
				i++
			}
			stack = append(stack, extended)
			out.WriteString(source[start:i])
		case ')':
			depth := len(stack)
			if depth > 0 {
				extended = stack[depth-1]
				stack = stack[:depth-1]
			}
			if closer, ok := closerAt[depth]; ok && depth > 0 {
				delete(closerAt, depth)
				out.WriteString(closer)
			} else {
				out.WriteByte(ch)
			}
			i++
		case '|', '*', '+', '?':
			out.WriteByte(ch)
			i++
		case '{':
			if end := strings.IndexByte(source[i:], '}'); end >= 0 && isRegexCountQuantifier(source[i+1:i+end]) {
				i += end + 1
				out.WriteString(source[start:i])
			} else {
				out.WriteString(wrap(`\{`, true))
				i++
			}
		case '[':
			i++
			if i < len(source) && source[i] == '^' {
				i++
			}
			if i < len(source) && source[i] == ']' {
				i++
			}
			for i < len(source) {
				if source[i] == '\\' && i+1 < len(source) {
					control := source[i+1] == 'c'
					i += 2
					if control && i < len(source) {
						i++
					}
					continue
				}
				i++
				if source[i-1] == ']' {
					break
				}
			}
			out.WriteString(wrap(source[start:i], true))
		case '\\':
			i += 2
			if i > len(source) {
				i = len(source)
			}
			if i < len(source) {
				switch source[start+1] {
				case 'p', 'P':
					if source[i] == '{' {
						if end := strings.IndexByte(source[i:], '}'); end >= 0 {
							i += end + 1
						}
					}
				case 'k':
					if source[i] == '<' {
						if end := strings.IndexByte(source[i:], '>'); end >= 0 {
							i += end + 1
						}
					}
				case 'u', 'x':
					count := 2
					if source[start+1] == 'u' {
						count = 4
					}
					for count > 0 && i < len(source) {
						i++
						count--
					}
				case 'c':
					i++
				default:
					if source[start+1] >= '0' && source[start+1] <= '9' {
						for i < len(source) && source[i] >= '0' && source[i] <= '9' {
							i++
						}
					}
				}
			}
			atom := source[start:i]
			switch atom {
			case `\A`, `\G`:
				out.WriteString(wrap(atom, false))
			case `\z`:
				out.WriteString(wrap(mark, false))
			case `\b`, `\B`:
				out.WriteString(wrap(atom+"(?:"+mark+")?", true))
			default:
				out.WriteString(wrap(atom, true))
			}
		case '^':
			out.WriteString(wrap("^", false))
			i++
		default:
			_, size := utf8.DecodeRuneInString(source[i:])
			i += size
			out.WriteString(wrap(source[start:i], true))
		}
	}
	return out.String(), slot
}

func (p *regexp2Plan) matchStartingAt(input string, startByte, endRune int, full bool) (*regexp2.Match, error) {
	// Put the end condition inside the expression so reluctant repetitions and
	// alternatives can backtrack to a full-region result (R028).
	source := `\G(?:` + p.matchSource + ")"
	if full {
		source += fmt.Sprintf(`(?=(?s:.{%d})\z)`, utf8.RuneCountInString(input)-endRune)
	}
	re, err := regexp2.Compile(source, regexp2.None)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = regexp2MatchTimeout
	if p.noLinearMatch(input) {
		return nil, nil
	}
	match, err := searchRegexp2(re, input, func() (*regexp2.Match, error) {
		return re.FindStringMatchStartingAt(input, startByte)
	})
	for match != nil && err == nil && !p.validGraphemeMatch(input, match) {
		previous := match
		match, err = searchRegexp2(re, input, func() (*regexp2.Match, error) {
			return re.FindNextMatch(previous)
		})
	}
	if match != nil && err == nil {
		if depthErr := p.checkStackDepth(match); depthErr != nil {
			return nil, depthErr
		}
	}
	return match, err
}

func (p *regexp2Plan) operationHitEnd(input string, startByte, endRune int, anchored, full bool) (bool, error) {
	if !anchored && p.noLinearMatch(input) {
		// A failed unanchored find ends in Java's Start node, which sets
		// hitEnd (pattern_matcher R011, d14_neg_lookahead_nomatch). A pattern
		// whose root node is a bare ^ has no Start node (a group around it does):
		// Java makes one attempt, and hitEnd is true when that attempt reads
		// past the end. The linear program decides that exactly.
		if p.linearProg == nil || !regexRootIsBareCaret(p.originalSource) {
			return true, nil
		}
		if startByte == 0 {
			return linearAttemptReachesEnd(p.linearProg, input, endRune), nil
		}
	}
	source, slot := regexp2EndObserverSource(p.matchSource, p.re)
	if anchored {
		source = `\G(?:` + source + ")"
	}
	if full {
		source += fmt.Sprintf(`(?=(?s:.{%d})\z)`, utf8.RuneCountInString(input)-endRune)
	}
	re, err := regexp2.Compile(source, regexp2.None)
	if err != nil {
		return false, err
	}
	re.MatchTimeout = regexp2MatchTimeout
	match, err := searchRegexp2(re, input, func() (*regexp2.Match, error) {
		return re.FindStringMatchStartingAt(input, startByte)
	})
	if err != nil || match == nil {
		return false, err
	}
	group := match.GroupByNumber(slot)
	return group != nil && len(group.Captures) > 0, nil
}

// regexStripLeadingFlags removes leading (?flags) groups from a pattern.
func regexStripLeadingFlags(source string) string {
	for strings.HasPrefix(source, "(?") {
		end := strings.IndexByte(source, ')')
		if end < 0 || strings.Trim(source[2:end], "idmsucxU-") != "" {
			break
		}
		source = source[end+1:]
	}
	return source
}

// linearAttemptReachesEnd reports whether some path of prog, started at the
// beginning of input, consumes every rune up to endRune and is still alive.
// The program has no empty-width operation other than begin of text.
func linearAttemptReachesEnd(prog *syntax.Prog, input string, endRune int) bool {
	runes := []rune(input)
	if endRune < len(runes) {
		runes = runes[:endRune]
	}
	var current, next []uint32
	seen := make([]int, len(prog.Inst))
	for i := range seen {
		seen[i] = -1
	}
	var add func(list *[]uint32, pc uint32, flags syntax.EmptyOp, gen int)
	add = func(list *[]uint32, pc uint32, flags syntax.EmptyOp, gen int) {
		if seen[pc] == gen {
			return
		}
		seen[pc] = gen
		inst := &prog.Inst[pc]
		switch inst.Op {
		case syntax.InstAlt, syntax.InstAltMatch:
			add(list, inst.Out, flags, gen)
			add(list, inst.Arg, flags, gen)
		case syntax.InstCapture, syntax.InstNop:
			add(list, inst.Out, flags, gen)
		case syntax.InstEmptyWidth:
			if syntax.EmptyOp(inst.Arg)&^flags == 0 {
				add(list, inst.Out, flags, gen)
			}
		case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
			*list = append(*list, pc)
		}
	}
	prev := rune(-1)
	var first rune = -1
	if len(runes) > 0 {
		first = runes[0]
	}
	add(&current, uint32(prog.Start), syntax.EmptyOpContext(prev, first), 0)
	for i, r := range runes {
		next = next[:0]
		after := rune(-1)
		if i+1 < len(runes) {
			after = runes[i+1]
		}
		for _, pc := range current {
			inst := &prog.Inst[pc]
			if inst.MatchRune(r) {
				add(&next, inst.Out, syntax.EmptyOpContext(r, after), i+1)
			}
		}
		current, next = next, current
		if len(current) == 0 {
			return false
		}
	}
	return len(current) > 0
}

// regexRootIsBareCaret reports whether Java compiles source to a root Begin
// node: no top-level alternation, and the first element, after any flag-only
// groups, is a bare ^. A group around the ^ keeps Java's Start wrapper.
func regexRootIsBareCaret(source string) bool {
	source = javaRegexRemoveQuoting(source)
	if regexSourceHasInlineFlag(source, 'x') {
		stripped, ok := regexStripExtended(source)
		if !ok {
			return false
		}
		source = stripped
	}
	if !strings.HasPrefix(regexStripLeadingFlags(source), "^") {
		return false
	}
	depth := 0
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			i++
		case '[':
			// A ] right after [ or [^ is a literal; classes nest.
			classDepth := 1
			i++
			if i < len(source) && source[i] == '^' {
				i++
			}
			if i < len(source) && source[i] == ']' {
				i++
			}
			for ; i < len(source) && classDepth > 0; i++ {
				switch source[i] {
				case '\\':
					i++
				case '[':
					classDepth++
				case ']':
					classDepth--
				}
			}
			i--
		case '(':
			depth++
		case ')':
			depth--
		case '|':
			if depth == 0 {
				return false
			}
		}
	}
	return true
}

// regexStripExtended rewrites a (?x) pattern into the equivalent plain
// pattern, so RE2 can read it. It accepts only the exact, simple shape: x set
// by leading flag groups and never changed, only flat classes without
// whitespace or comments, ASCII text without NUL, and no carriage returns.
func regexStripExtended(src string) (string, bool) {
	var out strings.Builder
	leading := true
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c >= 0x80, c == '\r', c == 0:
			return "", false
		case c == '[':
			// A flat class holding no whitespace or comment is copied as it is.
			end := regexPlainClassEnd(src, i)
			if end < 0 {
				return "", false
			}
			out.WriteString(src[i : end+1])
			i = end
			leading = false
		case c == '\\':
			if i+1 >= len(src) {
				return "", false
			}
			n := src[i+1]
			switch {
			case n == ' ':
				out.WriteString("[ ]")
			case n >= '1' && n <= '9' || strings.IndexByte("QEpPcN", n) >= 0 || n == '\t' || n == '\n' || n == '\v' || n == '\f' || n >= 0x80:
				return "", false
			case n == 'k':
				// \k<name> reads its name raw; accept only a plain name.
				end := strings.IndexByte(src[i:], '>')
				if i+2 >= len(src) || src[i+2] != '<' || end < 0 {
					return "", false
				}
				for _, nc := range src[i+3 : i+end] {
					if !(nc >= 'a' && nc <= 'z' || nc >= 'A' && nc <= 'Z' || nc >= '0' && nc <= '9') {
						return "", false
					}
				}
				out.WriteString(src[i : i+end+1])
				i += end
				leading = false
				continue
			default:
				out.WriteByte(c)
				out.WriteByte(n)
			}
			i++
			leading = false
		case c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f':
		case c == '#':
			for i < len(src) && src[i] != '\n' {
				if src[i] == '\r' || src[i] == 0 || src[i] >= 0x80 {
					return "", false
				}
				i++
			}
		case c == '(' && i+1 < len(src) && src[i+1] == '?':
			if i+2 < len(src) && src[i+2] == '#' {
				return "", false
			}
			j := i + 2
			for j < len(src) && strings.IndexByte("idmsuxU-", src[j]) >= 0 {
				j++
			}
			if j >= len(src) || (src[j] != ')' && src[j] != ':') {
				out.WriteString("(?")
				i++
				leading = false
				continue
			}
			flags := src[i+2 : j]
			if strings.Contains(flags, "x") {
				if !leading || src[j] != ')' || strings.Contains(flags, "-") {
					return "", false
				}
				flags = strings.ReplaceAll(flags, "x", "")
				if flags == "" {
					i = j
					continue
				}
			}
			out.WriteString("(?" + flags + string(src[j]))
			i = j
			if src[j] == ':' {
				leading = false
			}
		default:
			out.WriteByte(c)
			leading = false
		}
	}
	return out.String(), true
}

// regexNormalizeExtended returns the plain form of a simple (?x) pattern, or
// source itself when the pattern is not simple or the plain form is invalid.
func regexNormalizeExtended(source string) string {
	if !regexSourceHasInlineFlag(source, 'x') {
		return source
	}
	stripped, ok := regexStripExtended(javaRegexRemoveQuoting(source))
	if !ok || javaRegexSyntaxCheck(stripped) != nil {
		return source
	}
	return stripped
}

// regexPlainClassEnd returns the index of the ] that closes the class opened
// at start, or -1 unless the class is flat (no nesting, no &&, no leading ])
// and holds no whitespace, # or non-ASCII byte, so x mode changes nothing in it.
func regexPlainClassEnd(src string, start int) int {
	i := start + 1
	if i < len(src) && src[i] == '^' {
		i++
	}
	if i < len(src) && src[i] == ']' {
		return -1
	}
	for ; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\\':
			i++
			if i >= len(src) || src[i] == ' ' || src[i] < 0x20 || src[i] >= 0x80 || src[i] == 'Q' {
				return -1
			}
		case c == ']':
			return i
		case c == '[', c == '#', c == ' ', c < 0x20, c >= 0x80, c == '&' && i+1 < len(src) && src[i+1] == '&':
			return -1
		}
	}
	return -1
}
