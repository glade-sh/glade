package vm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dlclark/regexp2/syntax"
)

// Validate Java group names before the engine accepts its broader group syntax.
// R083-R085 record the native diagnostics, including the original source index.
func validateJavaRegexSource(source, exceptionType string) error {
	if se := javaRegexSyntaxCheck(source); se != nil {
		return newExceptionError("System."+exceptionType, javaRegexSyntaxMessage(source, se.description, se.index))
	}
	return nil
}

func validateJavaRegexGroups(source, exceptionType string) error {
	names := map[string]bool{}
	var classes javaRegexClassState
	quoted := false
	extended := false
	var stack []bool
	for i := 0; i < len(source); i++ {
		if source[i] == '\\' && i+1 < len(source) {
			if source[i+1] == 'Q' {
				quoted = true
			} else if source[i+1] == 'E' {
				quoted = false
			}
			i++
			if source[i] == 'c' && i+1 < len(source) {
				i++
			}
			continue
		}
		if quoted {
			continue
		}
		if extended && !classes.inside() && source[i] == '#' {
			if end := strings.IndexByte(source[i:], '\n'); end >= 0 {
				i += end
				continue
			}
			break
		}
		classes.advance(source, i)
		if classes.inside() || !strings.HasPrefix(source[i:], "(?") || i+2 >= len(source) {
			if !classes.inside() && source[i] == '(' {
				stack = append(stack, extended)
			} else if !classes.inside() && source[i] == ')' && len(stack) > 0 {
				extended = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if strings.HasPrefix(source[i:], "(?#") {
			if end := strings.IndexByte(source[i+3:], ')'); end >= 0 {
				i += end + 3
				continue
			}
		}
		j, scoped, enabled, sawFlag := i+2, extended, true, false
		for j < len(source) && strings.ContainsRune("idmsuxU-", rune(source[j])) {
			if source[j] == '-' {
				enabled = false
			} else {
				sawFlag = true
				if source[j] == 'x' {
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
			i = j
			continue
		}
		stack = append(stack, extended)
		if strings.HasPrefix(source[i:], "(?<") && i+3 < len(source) && source[i+3] != '=' && source[i+3] != '!' {
			end := strings.IndexByte(source[i+3:], '>')
			if end < 0 {
				continue
			}
			end += i + 3
			name := source[i+3 : end]
			index, description := i+3, "capturing group name does not start with a Latin letter"
			if len(name) > 0 && ((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z')) {
				if !names[name] {
					names[name] = true
					continue
				}
				index, description = end, "Named capturing group <"+name+"> is already defined"
			}
			return newExceptionError("System."+exceptionType, javaRegexSyntaxMessage(source, description, apexStringLength(source[:index])))
		}
		next := source[i+2]
		if strings.ContainsRune(":=!<>#P", rune(next)) {
			continue
		}
		for j := i + 2; j < len(source) && source[j] != ')' && source[j] != ':'; j++ {
			if !strings.ContainsRune("idmsuxU-", rune(source[j])) {
				return newExceptionError("System."+exceptionType, javaRegexSyntaxMessage(source, "Unknown inline modifier", apexStringLength(source[:j])))
			}
		}
	}
	return nil
}

// Native diagnostics retain the complete source and a caret for an index
// within it. Tabs remain tabs in the indentation (R078-R086, K024/K025).
func javaRegexSyntaxMessage(pattern, description string, index int) string {
	message := fmt.Sprintf("Invalid regex: %s near index %d\n%s", description, index, pattern)
	if index < 0 || index >= apexStringLength(pattern) {
		return message
	}
	var indent strings.Builder
	position := 0
	for _, char := range pattern {
		if position == index {
			break
		}
		if char == '\t' {
			indent.WriteByte('\t')
		} else {
			indent.WriteByte(' ')
		}
		position++
	}
	return message + "\n" + indent.String() + "^"
}

func javaRegexSyntaxDescription(pattern string, err error, fallback string) string {
	var parseErr *syntax.Error
	if !errors.As(err, &parseErr) {
		return fallback
	}
	index, description := -1, ""
	switch parseErr.Code {
	case syntax.ErrMissingParen, syntax.ErrUnterminatedBracket:
		// Preserve the complete excerpt from newRegexSyntaxError.
		return fallback
	case syntax.ErrIllegalEndEscape:
		index, description = apexStringLength(pattern), "Unescaped trailing backslash"
	case syntax.ErrInvalidRepeatSize:
		if end := strings.LastIndexByte(pattern, '}'); end >= 0 {
			index, description = apexStringLength(pattern[:end]), "Illegal repetition range"
		}
	case syntax.ErrReversedCharRange, syntax.ErrInvalidCharRange:
		for i := 1; i+1 < len(pattern); i++ {
			if pattern[i] == '-' && !isEscapedRegexByte(pattern, i) {
				index, description = utf8.RuneCountInString(pattern[:i+1]), "Illegal character range"
				break
			}
		}
	}
	if index >= 0 {
		return javaRegexSyntaxMessage(pattern, description, index)
	}
	return fallback
}

func matcherReplacementError(replacement string, err error) error {
	var groupErr *stringReplacementGroupError
	if errors.As(err, &groupErr) {
		return newExceptionError("System.StringException", fmt.Sprintf("No group %d", groupErr.group))
	}
	description := err.Error()
	description += "  (Invalid replacement string: '" + replacement + "'.  '$' should be escaped if you do not intend to use a backreference.  Try using Matcher.quoteReplacement() for your argument)"
	return newExceptionError("System.StringException", description)
}

// validateJavaRegexProperties rejects \p{Name} and \P{Name} the way Pattern.family
// does, reporting the description and the index Java reports.
func validateJavaRegexProperties(source, exceptionType string) error {
	if !strings.Contains(source, `\p`) && !strings.Contains(source, `\P`) {
		return nil
	}
	_, unicodeClass := rewriteInlineUnicodeCharacterClassFlagForRegexp2(source, false)
	fail := func(description string, index int) error {
		return newExceptionError("System."+exceptionType, javaRegexSyntaxMessage(source, description, index))
	}
	quoted := false
	for i := 0; i < len(source); i++ {
		if source[i] != '\\' || i+1 >= len(source) {
			continue
		}
		next := source[i+1]
		if quoted {
			if next == 'E' {
				quoted = false
			}
			i++
			continue
		}
		if next == 'Q' {
			quoted = true
			i++
			continue
		}
		i++
		if next != 'p' && next != 'P' {
			continue
		}
		if i+1 >= len(source) {
			return fail("Unknown character property name {}", apexStringLength(source))
		}
		if source[i+1] != '{' {
			r, _ := utf8.DecodeRuneInString(source[i+1:])
			name := string(r)
			if javaNativeCategoryNames[name] {
				continue
			}
			if _, perr := javaPropertySet(name, unicodeClass); perr != nil {
				return fail(perr.description, apexStringLength(source[:i+1]))
			}
			continue
		}
		end := strings.IndexByte(source[i+2:], '}')
		if end < 0 {
			return fail("Unclosed character family", apexStringLength(source))
		}
		end += i + 2
		name := source[i+2 : end]
		if name == "" {
			return fail("Empty character family", apexStringLength(source[:end]))
		}
		if !javaNativeCategoryNames[name] {
			if _, perr := javaPropertySet(name, unicodeClass); perr != nil {
				return fail(perr.description, apexStringLength(source[:end]))
			}
		}
		i = end
	}
	return nil
}

// javaCapturingGroupCount counts capturing groups in the order Java numbers them.
func javaCapturingGroupCount(source string) int {
	count := 0
	var classes javaRegexClassState
	for i := 0; i < len(source); i++ {
		if source[i] == '\\' {
			i++
			continue
		}
		classes.advance(source, i)
		if classes.inside() || source[i] != '(' {
			continue
		}
		if i+1 < len(source) && source[i+1] == '?' {
			if i+3 < len(source) && source[i+2] == '<' && source[i+3] != '=' && source[i+3] != '!' {
				count++
			}
			continue
		}
		count++
	}
	return count
}

// rewriteJavaNumericEscapesForRegexp2 resolves the escapes whose meaning Java
// and regexp2 disagree on: \0nn octal sequences and numeric back references.
// Java extends \N while the longer number names a group opened so far, treats
// \0 as octal, and lets a reference to a missing group fail to match.
func rewriteJavaNumericEscapesForRegexp2(source string) string {
	if !strings.Contains(source, `\`) {
		return source
	}
	total := javaCapturingGroupCount(source)
	var out strings.Builder
	var classes javaRegexClassState
	opened := 0
	changed := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch != '\\' {
			classes.advance(source, i)
			if !classes.inside() && ch == '(' {
				if i+1 < len(source) && source[i+1] == '?' {
					if i+3 < len(source) && source[i+2] == '<' && source[i+3] != '=' && source[i+3] != '!' {
						opened++
					}
				} else {
					opened++
				}
			}
			out.WriteByte(ch)
			continue
		}
		if i+1 >= len(source) {
			out.WriteByte(ch)
			continue
		}
		next := source[i+1]
		switch {
		case next == '0':
			value, used := javaOctalEscapeValue(source[i+2:])
			if used == 0 {
				out.WriteString(source[i : i+2])
				i++
				continue
			}
			fmt.Fprintf(&out, `\u%04X`, value)
			i += 1 + used
			changed = true
		case next >= '1' && next <= '9' && !classes.inside():
			number := int(next - '0')
			j := i + 2
			for j < len(source) && source[j] >= '0' && source[j] <= '9' {
				candidate := number*10 + int(source[j]-'0')
				if opened < candidate {
					break
				}
				number = candidate
				j++
			}
			switch {
			case number > total:
				out.WriteString(`(?!)`)
			case j < len(source) && source[j] >= '0' && source[j] <= '9':
				fmt.Fprintf(&out, `(?:\%d)`, number)
			default:
				fmt.Fprintf(&out, `\%d`, number)
			}
			i = j - 1
			changed = true
		case next == 'u' && i+11 < len(source)+0 && source[i+6] == '\\' && source[i+7] == 'u':
			// Java joins an escaped surrogate pair into one code point (Pattern.u).
			high, okHigh := parseJavaHexQuad(source[i+2 : i+6])
			low, okLow := parseJavaHexQuad(source[i+8 : i+12])
			if okHigh && okLow && high >= 0xD800 && high <= 0xDBFF && low >= 0xDC00 && low <= 0xDFFF {
				out.WriteRune(utf16.DecodeRune(high, low))
				i += 11
				changed = true
				continue
			}
			out.WriteByte(ch)
			out.WriteByte(next)
			i++
		case next == 'N' && i+2 < len(source) && source[i+2] == '{':
			end := strings.IndexByte(source[i+3:], '}')
			if end >= 0 {
				if r, ok := javaCharacterByName(source[i+3 : i+3+end]); ok {
					fmt.Fprintf(&out, `\u%04X`, r)
					i += 3 + end
					changed = true
					continue
				}
			}
			out.WriteByte(ch)
			out.WriteByte(next)
			i++
		default:
			out.WriteByte(ch)
			out.WriteByte(next)
			i++
		}
	}
	if !changed {
		return source
	}
	return out.String()
}

// javaOctalEscapeValue reads the digits after \0 as java.util.regex does and
// returns the code point and how many source bytes follow the zero.
func javaOctalEscapeValue(rest string) (rune, int) {
	isOctalByte := func(b byte) bool { return b >= '0' && b <= '7' }
	if len(rest) == 0 || !isOctalByte(rest[0]) {
		return 0, 0
	}
	n := rune(rest[0] - '0')
	if len(rest) < 2 || !isOctalByte(rest[1]) {
		return n, 1 + 1 - 1
	}
	m := rune(rest[1] - '0')
	if len(rest) >= 3 && isOctalByte(rest[2]) && n <= 3 {
		return n*64 + m*8 + rune(rest[2]-'0'), 3
	}
	return n*8 + m, 2
}

// rewriteJavaStackedQuantifiersForRegexp2 groups an atom that already has a
// quantifier before a following {n,m}: Java reads a{1,2}{3} as (?:a{1,2}){3},
// while regexp2 rejects the nested repetition.
func rewriteJavaStackedQuantifiersForRegexp2(source string) string {
	if !strings.Contains(source, "}{") && !strings.Contains(source, "*{") && !strings.Contains(source, "+{") && !strings.Contains(source, "?{") {
		return source
	}
	var out []byte
	var groups []int
	atomStart := -1
	quantified := false
	for i := 0; i < len(source); {
		ch := source[i]
		switch {
		case ch == '\\' && i+1 < len(source):
			size := 2
			switch source[i+1] {
			case 'p', 'P', 'x', 'k':
				if i+2 < len(source) && (source[i+2] == '{' || source[i+2] == '<') {
					closer := byte('}')
					if source[i+2] == '<' {
						closer = '>'
					}
					if end := strings.IndexByte(source[i:], closer); end >= 0 {
						size = end + 1
					}
				}
			case 'c':
				size = 3
			}
			if i+size > len(source) {
				size = len(source) - i
			}
			atomStart, quantified = len(out), false
			out = append(out, source[i:i+size]...)
			i += size
		case ch == '[':
			end := javaRegexCharClassEnd(source, i)
			if end < 0 {
				return source
			}
			atomStart, quantified = len(out), false
			out = append(out, source[i:end+1]...)
			i = end + 1
		case ch == '(':
			groups = append(groups, len(out))
			quantified = false
			out = append(out, ch)
			i++
		case ch == ')':
			if len(groups) > 0 {
				atomStart = groups[len(groups)-1]
				groups = groups[:len(groups)-1]
			}
			quantified = false
			out = append(out, ch)
			i++
		case ch == '*' || ch == '+' || ch == '?':
			out = append(out, ch)
			i++
			if i < len(source) && (source[i] == '?' || source[i] == '+') && ch != '?' || i < len(source) && ch == '?' && (source[i] == '?' || source[i] == '+') {
				out = append(out, source[i])
				i++
			}
			quantified = true
		case ch == '{':
			end := strings.IndexByte(source[i:], '}')
			if end < 0 || !isRegexCountQuantifier(source[i+1:i+end]) {
				atomStart, quantified = len(out), false
				out = append(out, ch)
				i++
				break
			}
			if quantified && atomStart >= 0 {
				wrapped := append([]byte("(?:"), out[atomStart:]...)
				wrapped = append(wrapped, ')')
				out = append(out[:atomStart], wrapped...)
			}
			out = append(out, source[i:i+end+1]...)
			i += end + 1
			if i < len(source) && (source[i] == '?' || source[i] == '+') {
				out = append(out, source[i])
				i++
			}
			quantified = true
		default:
			_, size := utf8.DecodeRuneInString(source[i:])
			atomStart, quantified = len(out), false
			out = append(out, source[i:i+size]...)
			i += size
		}
	}
	return string(out)
}

func parseJavaHexQuad(text string) (rune, bool) {
	if len(text) != 4 {
		return 0, false
	}
	v, err := strconv.ParseUint(text, 16, 16)
	return rune(v), err == nil
}
