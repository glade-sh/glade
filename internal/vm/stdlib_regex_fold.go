package vm

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Java folds case for ASCII letters only. With the u flag (UNICODE_CASE) it
// folds through the simple case mappings: a pattern character c matches ch when
// lower(upper(c)) == lower(upper(ch)), except that a character whose upper and
// lower(upper) forms are equal matches only itself (Pattern.single). These
// helpers expand the folds into plain classes, so regexp2's own folding, which
// differs for non-ASCII text, is not used.

var (
	javaFoldOnce   sync.Once
	javaFoldCased  []rune
	javaFoldGroups map[rune][]rune
)

func javaFoldInit() {
	javaFoldOnce.Do(func() {
		javaFoldGroups = map[rune][]rune{}
		for r := rune(0); r <= unicode.MaxRune; r++ {
			if r >= 0xD800 && r <= 0xDFFF {
				continue
			}
			upper := unicode.ToUpper(r)
			lower := unicode.ToLower(upper)
			if upper == r && lower == r && unicode.ToLower(r) == r {
				continue
			}
			javaFoldCased = append(javaFoldCased, r)
			javaFoldGroups[lower] = append(javaFoldGroups[lower], r)
		}
	})
}

// javaFoldVariants lists every character Java's case-insensitive match treats
// as equal to c, c first. A single entry means c folds to nothing else.
func javaFoldVariants(c rune, unicodeCase bool) []rune {
	if !unicodeCase {
		switch {
		case c >= 'a' && c <= 'z':
			return []rune{c, c - 32}
		case c >= 'A' && c <= 'Z':
			return []rune{c, c + 32}
		}
		return []rune{c}
	}
	upper := unicode.ToUpper(c)
	lower := unicode.ToLower(upper)
	if upper == lower {
		return []rune{c}
	}
	javaFoldInit()
	out := []rune{c}
	seen := map[rune]bool{c: true}
	add := func(r rune) {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	add(lower)
	for _, r := range javaFoldGroups[lower] {
		add(r)
	}
	return out
}

// javaFoldRange lists the extra characters a case-insensitive range admits.
func javaFoldRange(lo, hi rune, unicodeCase bool) []rune {
	var extra []rune
	inRange := func(r rune) bool { return r >= lo && r <= hi }
	if !unicodeCase {
		for _, base := range []rune{'a', 'A'} {
			for k := rune(0); k < 26; k++ {
				r := base + k
				swapped := r ^ 0x20
				if !inRange(r) && inRange(swapped) {
					extra = append(extra, r)
				}
			}
		}
		return extra
	}
	javaFoldInit()
	for _, r := range javaFoldCased {
		if inRange(r) {
			continue
		}
		upper := unicode.ToUpper(r)
		if inRange(upper) || inRange(unicode.ToLower(upper)) {
			extra = append(extra, r)
		}
	}
	// Characters that map into the range from outside the cased list.
	for r := lo; r <= hi && hi-lo < 0x3000; r++ {
		for _, v := range javaFoldVariants(r, true) {
			if !inRange(v) {
				extra = append(extra, v)
			}
		}
	}
	return extra
}

func runeSetClassFromList(list []rune) string {
	pairs := make(runeSet, 0, len(list)*2)
	for _, r := range list {
		pairs = append(pairs, r, r)
	}
	return runeSetClass(normalizeRuneSet(pairs))
}

func fmtRuneEscape(r rune) string {
	if r < 0x80 {
		return fmt.Sprintf(`\x%02X`, r)
	}
	if r <= 0xFFFF {
		return fmt.Sprintf(`\u%04X`, r)
	}
	return string(r)
}

// decodeJavaEscapeChar reads \uXXXX, \xHH or \x{H...} at the start of s.
func decodeJavaEscapeChar(s string) (rune, int, bool) {
	if len(s) < 2 || s[0] != '\\' {
		return 0, 0, false
	}
	parseHex := func(digits string) (rune, bool) {
		v, err := strconv.ParseUint(digits, 16, 32)
		return rune(v), err == nil && v <= unicode.MaxRune
	}
	switch s[1] {
	case 'u':
		if len(s) >= 6 {
			if v, ok := parseHex(s[2:6]); ok {
				if v >= 0xD800 && v <= 0xDFFF {
					return 0, 0, false
				}
				return v, 6, true
			}
		}
	case 'x':
		if len(s) >= 4 && s[2] != '{' {
			if v, ok := parseHex(s[2:4]); ok {
				return v, 4, true
			}
		}
		if len(s) >= 4 && s[2] == '{' {
			if end := strings.IndexByte(s, '}'); end > 3 {
				if v, ok := parseHex(s[3:end]); ok {
					return v, end + 1, true
				}
			}
		}
	}
	return 0, 0, false
}

// foldEscapeAtom folds a character escape that stands alone in the pattern.
func foldEscapeAtom(s string, unicodeCase bool) (int, string, bool) {
	r, n, ok := decodeJavaEscapeChar(s)
	if !ok {
		return 0, "", false
	}
	variants := javaFoldVariants(r, unicodeCase)
	if len(variants) < 2 {
		return n, s[:n], true
	}
	return n, runeSetClassFromList(variants), true
}

// foldClassText adds the case variants to every literal and range of a class.
func foldClassText(class string, unicodeCase, extended bool) string {
	var out strings.Builder
	out.WriteByte('[')
	body := class[1 : len(class)-1]
	var extras []rune
	i := 0
	if strings.HasPrefix(body, "^") {
		out.WriteByte('^')
		i++
	}
	first := true
	// item reads one literal or escape. It returns the rune, its source length,
	// and false when the item is a set escape that stays untouched.
	item := func(at int) (rune, int, bool) {
		if body[at] == '\\' {
			if r, n, ok := decodeJavaEscapeChar(body[at:]); ok {
				return r, n, true
			}
			if at+1 < len(body) {
				next := body[at+1]
				if next == 'p' || next == 'P' {
					if at+2 < len(body) && body[at+2] == '{' {
						if end := strings.IndexByte(body[at:], '}'); end >= 0 {
							return 0, end + 1, false
						}
					}
					return 0, 3, false
				}
				if next < 0x80 && (next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z' || next >= '0' && next <= '9') {
					if strings.ContainsRune("tnrfae", rune(next)) {
						return map[byte]rune{'t': '\t', 'n': '\n', 'r': '\r', 'f': '\f', 'a': 7, 'e': 0x1B}[next], 2, true
					}
					return 0, 2, false
				}
				r, size := utf8.DecodeRuneInString(body[at+1:])
				return r, 1 + size, true
			}
			return '\\', 1, true
		}
		r, size := utf8.DecodeRuneInString(body[at:])
		return r, size, true
	}
	for i < len(body) {
		c := body[i]
		switch {
		case c == '[':
			end := javaClassNestedEnd(body, i)
			if end < 0 {
				out.WriteString(body[i:])
				i = len(body)
				break
			}
			out.WriteString(foldClassText(body[i:end+1], unicodeCase, extended))
			i = end + 1
			first = false
		case c == '&' && i+1 < len(body) && body[i+1] == '&':
			out.WriteString("&&")
			i += 2
		case extended && javaASCIISpace(rune(c)):
			i++
		case extended && c == '#':
			if end := strings.IndexByte(body[i:], '\n'); end >= 0 {
				i += end
			} else {
				i = len(body)
			}
		default:
			r, n, literal := item(i)
			if !literal {
				out.WriteString(body[i : i+n])
				i += n
				first = false
				break
			}
			// A hyphen that does not follow a literal is itself a literal.
			if i+n < len(body) && body[i+n] == '-' && i+n+1 < len(body) && body[i+n+1] != ']' && !(first && c == '-') {
				if hi, m, ok := item(i + n + 1); ok {
					out.WriteString(body[i : i+n+1+m])
					extras = append(extras, javaFoldRange(r, hi, unicodeCase)...)
					i += n + 1 + m
					first = false
					break
				}
			}
			out.WriteString(body[i : i+n])
			if variants := javaFoldVariants(r, unicodeCase); len(variants) > 1 {
				extras = append(extras, variants[1:]...)
			}
			i += n
			first = false
		}
	}
	if len(extras) > 0 {
		sort.Slice(extras, func(a, b int) bool { return extras[a] < extras[b] })
		var prev rune = -1
		for _, r := range extras {
			if r == prev {
				continue
			}
			prev = r
			out.WriteString(fmtRuneEscape(r))
		}
	}
	out.WriteByte(']')
	return out.String()
}
