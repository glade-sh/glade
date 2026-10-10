package vm

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2/syntax"
)

// Validate Java group names before the engine accepts its broader group syntax.
// R083-R085 record the native diagnostics, including the original source index.
func validateJavaRegexGroups(source, exceptionType string) error {
	if exceptionType != "StringException" {
		return nil
	}
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
	switch {
	case description == "replacement trailing escape":
		description = "character to be escaped is missing"
	case description == "replacement missing group reference after $":
		description = "Illegal group reference: group index is missing"
	case strings.Contains(description, "replacement named group references"):
		start := strings.Index(replacement, "${")
		if start >= 0 {
			end := strings.IndexByte(replacement[start+2:], '}')
			if end >= 0 {
				description = "No group with name {" + replacement[start+2:start+2+end] + "}"
			}
		}
	}
	description += "  (Invalid replacement string: '" + replacement + "'.  '$' should be escaped if you do not intend to use a backreference.  Try using Matcher.quoteReplacement() for your argument)"
	return newExceptionError("System.StringException", description)
}
