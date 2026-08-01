package vm

import (
	"fmt"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

const regexp2MatchTimeout = 2 * time.Second

func compileRegexp2Pattern(callee, source string) (string, *regexp2.Regexp, error) {
	regexp2Source, err := compileRegexp2Source(callee, source)
	if err != nil {
		return "", nil, err
	}
	compileSource := regexp2CompileSourceForSyntax(regexp2Source)
	re, err := regexp2.Compile(compileSource, regexp2.None)
	if err != nil {
		return "", nil, newPatternSyntaxExceptionError(source, err)
	}
	re.MatchTimeout = regexp2MatchTimeout
	return regexp2Source, re, nil
}

func compileRegexp2Source(callee, source string) (string, error) {
	converted, err := javaRegexQuoteEscapesToGo(source)
	if err != nil {
		return "", unsupportedCallError(callee + " " + err.Error())
	}
	regexp2Source := converted
	unicodeCharacterClass := false
	regexp2Source, unicodeCharacterClass = rewriteInlineUnicodeCharacterClassFlagForRegexp2(regexp2Source, unicodeCharacterClass)
	regexp2Source = rewriteJavaRegexEscapesForRegexp2(regexp2Source)
	regexp2Source = rewriteJavaUnicodeClassesForRegexp2(regexp2Source)
	regexp2Source = rewriteJavaShorthandClassesForRegexp2(regexp2Source, unicodeCharacterClass)
	regexp2Source, err = rewriteJavaClassAlgebraForRegexp2(regexp2Source)
	if err != nil {
		return "", unsupportedCallError(callee + " " + err.Error())
	}
	regexp2Source = rewriteSimplePossessiveQuantifiersForRegexp2(regexp2Source)
	if feature := unsupportedRegexp2JavaRegexFeature(regexp2Source); feature != "" {
		return "", unsupportedCallError(callee + " " + feature)
	}
	return regexp2Source, nil
}

func unsupportedRegexp2JavaRegexFeature(source string) string {
	inClass := false
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			if i+1 < len(source) {
				next := source[i+1]
				if next == 'Q' || next == 'E' {
					return "Java regex quote escapes"
				}
				if (next == 'p' || next == 'P') && i+2 < len(source) && source[i+2] == '{' {
					end := strings.IndexByte(source[i+3:], '}')
					if end >= 0 {
						className := source[i+3 : i+3+end]
						if javaOnlyUnicodeClass(className) {
							return "Java regex Unicode character classes"
						}
					}
				}
				i++
			}
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '&':
			if inClass && i+1 < len(source) && source[i+1] == '&' {
				return "Java regex character-class intersections"
			}
		case '(':
			if inClass {
				continue
			}
			if i+2 >= len(source) || source[i+1] != '?' {
				continue
			}
			switch source[i+2] {
			case 'P':
				if i+3 < len(source) && source[i+3] == '<' {
					return "Java regex named groups"
				}
			default:
				if unsupportedInlineJavaRegexFlags(source[i+2:]) {
					return "Java regex inline flags"
				}
			}
		case '*', '+', '?':
			if !inClass && i+1 < len(source) && source[i+1] == '+' {
				return "Java regex possessive quantifiers"
			}
		case '}':
			if !inClass && i+1 < len(source) && source[i+1] == '+' && regexQuantifierStart(source, i) >= 0 {
				return "Java regex possessive quantifiers"
			}
		}
	}
	return ""
}

func rewriteJavaRegexEscapesForRegexp2(source string) string {
	const horizontal = `\t \u00A0\u1680\u180E\u2000-\u200A\u202F\u205F\u3000`
	const vertical = `\n\v\f\r\x85\u2028\u2029`
	var out strings.Builder
	inClass := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		switch ch {
		case '\\':
			if i+1 >= len(source) {
				out.WriteByte(ch)
				continue
			}
			next := source[i+1]
			switch next {
			case 'R':
				if inClass {
					out.WriteString(`R`)
				} else {
					out.WriteString(`(?:\r\n|[` + vertical + `])`)
				}
				i++
			case 'X':
				out.WriteByte(ch)
				i++
				out.WriteByte(source[i])
			case 'h':
				if inClass {
					out.WriteString(horizontal)
				} else {
					out.WriteString(`[` + horizontal + `]`)
				}
				i++
			case 'H':
				if inClass {
					out.WriteString(`\H`)
				} else {
					out.WriteString(`[^` + horizontal + `]`)
				}
				i++
			case 'v':
				if inClass {
					out.WriteString(vertical)
				} else {
					out.WriteString(`[` + vertical + `]`)
				}
				i++
			case 'V':
				if inClass {
					out.WriteString(`\V`)
				} else {
					out.WriteString(`[^` + vertical + `]`)
				}
				i++
			default:
				out.WriteByte(ch)
				i++
				out.WriteByte(source[i])
			}
		case '[':
			inClass = true
			out.WriteByte(ch)
		case ']':
			inClass = false
			out.WriteByte(ch)
		default:
			out.WriteByte(ch)
		}
	}
	return out.String()
}

func rewriteInlineUnicodeCharacterClassFlagForRegexp2(source string, enabled bool) (string, bool) {
	var out strings.Builder
	inClass := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch == '\\' {
			out.WriteByte(ch)
			if i+1 < len(source) {
				i++
				out.WriteByte(source[i])
			}
			continue
		}
		switch ch {
		case '[':
			inClass = true
		case ']':
			inClass = false
		case '(':
			if !inClass && strings.HasPrefix(source[i:], "(?U)") {
				enabled = true
				i += len("(?U)") - 1
				continue
			}
		}
		out.WriteByte(ch)
	}
	return out.String(), enabled
}

func rewriteJavaUnicodeClassesForRegexp2(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		if source[i] != '\\' || i+2 >= len(source) || (source[i+1] != 'p' && source[i+1] != 'P') || source[i+2] != '{' {
			out.WriteByte(source[i])
			continue
		}
		end := strings.IndexByte(source[i+3:], '}')
		if end < 0 {
			out.WriteByte(source[i])
			continue
		}
		name := source[i+3 : i+3+end]
		if rewritten, ok := javaUnicodeClassEscapeForRegexp2(name, source[i+1] == 'p'); ok {
			out.WriteString(rewritten)
			i += 3 + end
			continue
		}
		out.WriteString(source[i : i+4+end])
		i += 3 + end
	}
	return out.String()
}

func javaUnicodeClassEscapeForRegexp2(name string, positive bool) (string, bool) {
	if strings.EqualFold(name, "javaWhitespace") {
		if positive {
			return `[\p{Zs}\t\n\x0B\f\r\x1C-\x1F]`, true
		}
		return `[^\p{Zs}\t\n\x0B\f\r\x1C-\x1F]`, true
	}
	if rewritten, ok := javaUnicodeClassForRegexp2(name); ok {
		prefix := `\p`
		if !positive {
			prefix = `\P`
		}
		return prefix + "{" + rewritten + "}", true
	}
	return "", false
}

func javaUnicodeClassForRegexp2(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "javalowercase":
		return "Ll", true
	case "javauppercase":
		return "Lu", true
	case "javadigit":
		return "Nd", true
	case "javaletter", "isalphabetic":
		return "L", true
	case "javacurrency":
		return "Sc", true
	}
	if strings.HasPrefix(name, "Is") && len(name) > 2 {
		return name[2:], true
	}
	return "", false
}

func rewriteJavaShorthandClassesForRegexp2(source string, unicodeCharacterClass bool) string {
	var out strings.Builder
	inClass := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if ch != '\\' || i+1 >= len(source) {
			switch ch {
			case '[':
				inClass = true
			case ']':
				inClass = false
			}
			out.WriteByte(ch)
			continue
		}
		next := source[i+1]
		replacement := javaShorthandClassForRegexp2(next, unicodeCharacterClass, inClass)
		if replacement == "" {
			out.WriteByte(ch)
			i++
			out.WriteByte(next)
			continue
		}
		out.WriteString(replacement)
		i++
	}
	return out.String()
}

func javaShorthandClassForRegexp2(ch byte, unicodeCharacterClass bool, inClass bool) string {
	if inClass {
		switch ch {
		case 'w':
			if unicodeCharacterClass {
				return `\p{L}\p{M}\p{Nd}\p{Pc}`
			}
			return `A-Za-z0-9_`
		case 'd':
			if unicodeCharacterClass {
				return `\p{Nd}`
			}
			return `0-9`
		case 's':
			if unicodeCharacterClass {
				return `\t\n\v\f\r\x{1C}-\x{1F}\x85\u2028\u2029\p{Zs}`
			}
			return ` \t\n\v\f\r`
		}
		return ""
	}
	if unicodeCharacterClass {
		switch ch {
		case 'w':
			return `[\p{L}\p{M}\p{Nd}\p{Pc}]`
		case 'W':
			return `[^\p{L}\p{M}\p{Nd}\p{Pc}]`
		case 'd':
			return `\p{Nd}`
		case 'D':
			return `\P{Nd}`
		case 's':
			return `[\t\n\v\f\r\x{1C}-\x{1F}\x85\u2028\u2029\p{Zs}]`
		case 'S':
			return `[^\t\n\v\f\r\x{1C}-\x{1F}\x85\u2028\u2029\p{Zs}]`
		}
		return ""
	}
	switch ch {
	case 'w':
		return `[A-Za-z0-9_]`
	case 'W':
		return `[^A-Za-z0-9_]`
	case 'd':
		return `[0-9]`
	case 'D':
		return `[^0-9]`
	case 's':
		return `[ \t\n\v\f\r]`
	case 'S':
		return `[^ \t\n\v\f\r]`
	}
	return ""
}

func rewriteSimplePossessiveQuantifiersForRegexp2(source string) string {
	var out strings.Builder
	last := 0
	inClass := false
	for i := 0; i < len(source)-1; i++ {
		ch := source[i]
		if ch == '\\' {
			i++
			continue
		}
		switch ch {
		case '[':
			inClass = true
			continue
		case ']':
			inClass = false
			continue
		}
		if inClass || source[i+1] != '+' {
			continue
		}
		quantStart := i
		if ch == '}' {
			start := regexQuantifierStart(source, i)
			if start < 0 {
				continue
			}
			quantStart = start
		} else if ch != '*' && ch != '+' && ch != '?' {
			continue
		}
		atomStart := regexAtomStart(source, quantStart)
		if atomStart < 0 {
			continue
		}
		out.WriteString(source[last:atomStart])
		out.WriteString("(?>")
		out.WriteString(source[atomStart : i+1])
		out.WriteByte(')')
		i++
		last = i + 1
	}
	out.WriteString(source[last:])
	return out.String()
}

func regexQuantifierStart(source string, end int) int {
	depth := 0
	for i := end - 1; i >= 0; i-- {
		if isEscapedRegexByte(source, i) {
			continue
		}
		switch source[i] {
		case '}':
			depth++
		case '{':
			if depth > 0 {
				depth--
				continue
			}
			if !isRegexCountQuantifier(source[i+1 : end]) {
				return -1
			}
			return i
		}
	}
	return -1
}

func isRegexCountQuantifier(body string) bool {
	sawDigit := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if ch >= '0' && ch <= '9' {
			sawDigit = true
			continue
		}
		if ch == ',' {
			continue
		}
		return false
	}
	return sawDigit
}

func regexAtomStart(source string, quantStart int) int {
	if quantStart <= 0 {
		return -1
	}
	end := quantStart - 1
	switch source[end] {
	case ']':
		return regexCharClassStart(source, end)
	case ')':
		return regexGroupStart(source, end)
	}
	if isEscapedRegexByte(source, end) {
		return end - 1
	}
	return end
}

func regexCharClassStart(source string, end int) int {
	nested := 0
	for i := end - 1; i >= 0; i-- {
		if isEscapedRegexByte(source, i) {
			continue
		}
		switch source[i] {
		case ']':
			nested++
		case '[':
			if nested > 0 {
				nested--
				continue
			}
			return i
		}
	}
	return -1
}

func regexGroupStart(source string, end int) int {
	depth := 0
	inClass := false
	for i := end - 1; i >= 0; i-- {
		if isEscapedRegexByte(source, i) {
			continue
		}
		switch source[i] {
		case ']':
			inClass = true
		case '[':
			inClass = false
		case ')':
			if !inClass {
				depth++
			}
		case '(':
			if inClass {
				continue
			}
			if depth > 0 {
				depth--
				continue
			}
			return i
		}
	}
	return -1
}

func matcherRegexp2Source(matcher Value) (string, error) {
	if source, ok := matcher.Fields["regexp2Source"]; ok {
		if source.Kind != ValueString {
			return "", fmt.Errorf("Matcher stored invalid regexp2 source")
		}
		return source.Text, nil
	}
	return matcherSourceOnly(matcher)
}

func matcherRegexp2PlanForInput(matcher Value, input string) (*regexp2Plan, error) {
	source, err := matcherOriginalRegexSource(matcher)
	if err != nil {
		return nil, err
	}
	return compileRegexp2PlanForInput("Pattern.compile", source, input)
}

func matcherOriginalRegexSource(matcher Value) (string, error) {
	if source, ok := matcher.Fields["patternSource"]; ok {
		if source.Kind != ValueString {
			return "", fmt.Errorf("Matcher stored invalid Pattern source")
		}
		return source.Text, nil
	}
	return matcherSourceOnly(matcher)
}

func patternRegexp2Source(pattern Value) (string, error) {
	if source, ok := pattern.Fields["regexp2Source"]; ok {
		if source.Kind != ValueString {
			return "", fmt.Errorf("Pattern stored invalid regexp2 source")
		}
		return source.Text, nil
	}
	source, ok := pattern.Fields["source"]
	if !ok || source.Kind != ValueString {
		return "", fmt.Errorf("Pattern missing source")
	}
	regexp2Source, _, err := compileRegexp2Pattern("Pattern.compile", source.Text)
	return regexp2Source, err
}

func patternSourceOnly(pattern Value) (string, error) {
	source, ok := pattern.Fields["source"]
	if !ok || source.Kind != ValueString {
		return "", fmt.Errorf("Pattern missing source")
	}
	return source.Text, nil
}

func matcherSourceOnly(receiver Value) (string, error) {
	source, ok := receiver.Fields["source"]
	if !ok || source.Kind != ValueString {
		return "", fmt.Errorf("Matcher missing Pattern source")
	}
	return source.Text, nil
}

func regexp2GroupCount(re *regexp2.Regexp) int {
	count := 0
	for _, number := range re.GetGroupNumbers() {
		if number > 0 {
			count++
		}
	}
	return count
}

func regexp2MatchByteIndices(input string, match *regexp2.Match, runeOffset int) ([]int, error) {
	groups := match.Groups()
	indices := make([]int, 0, len(groups)*2)
	for _, group := range groups {
		if len(group.Captures) == 0 {
			indices = append(indices, -1, -1)
			continue
		}
		startRune := runeOffset + group.Index
		endRune := startRune + group.Length
		startByte, err := byteIndexForRuneIndex(input, startRune)
		if err != nil {
			return nil, err
		}
		endByte, err := byteIndexForRuneIndex(input, endRune)
		if err != nil {
			return nil, err
		}
		indices = append(indices, startByte, endByte)
	}
	return indices, nil
}

func matcherRegexp2MatchIndices(matcher Value, input string, region matcherRegionBounds, op matcherOp) ([]int, error) {
	text := input
	runeOffset := 0
	startAt := region.startByte
	requiredStart := region.startRune
	requiredEnd := region.endRune
	if !matcherUsesFullInputBounds(matcher) {
		text = input[region.startByte:region.endByte]
		runeOffset = region.startRune
		startAt = 0
		requiredStart = 0
		requiredEnd = region.endRune - region.startRune
	}
	plan, err := matcherRegexp2PlanForInput(matcher, text)
	if err != nil {
		return nil, err
	}
	match, err := plan.findValidStartingAt(text, startAt)
	if err != nil || match == nil {
		return nil, err
	}
	if match.Index != requiredStart {
		return nil, nil
	}
	if op == matcherOpMatches && match.Index+match.Length != requiredEnd {
		return nil, nil
	}
	return plan.matchByteIndices(input, match, runeOffset)
}

func matcherRegexp2FindIndices(matcher Value, input string, region matcherRegionBounds, startByte int) ([]int, error) {
	if !matcherUsesFullInputBounds(matcher) {
		if startByte < region.startByte {
			startByte = region.startByte
		}
		text := input[region.startByte:region.endByte]
		plan, err := matcherRegexp2PlanForInput(matcher, text)
		if err != nil {
			return nil, err
		}
		match, err := plan.findValidStartingAt(text, startByte-region.startByte)
		if err != nil || match == nil {
			return nil, err
		}
		return plan.matchByteIndices(input, match, region.startRune)
	}
	plan, err := matcherRegexp2PlanForInput(matcher, input)
	if err != nil {
		return nil, err
	}
	match, err := plan.findValidStartingAt(input, startByte)
	for match != nil && err == nil {
		start := match.Index
		end := match.Index + match.Length
		if start < region.startRune {
			match, err = plan.findNextValid(input, match)
			continue
		}
		if start > region.endRune {
			return nil, nil
		}
		if end <= region.endRune {
			return plan.matchByteIndices(input, match, 0)
		}
		match, err = plan.findNextValid(input, match)
	}
	return nil, err
}

func splitRegexRegexp2(name, pattern, text string, limit int64) ([]string, error) {
	plan, err := compileRegexp2PlanForInput(name, pattern, text)
	if err != nil {
		return nil, err
	}
	if limit == 1 {
		return []string{text}, nil
	}
	textRunes := []rune(text)
	var parts []string
	lastEnd := 0
	splits := int64(0)
	match, err := plan.findValidStartingAt(text, 0)
	for match != nil {
		if limit > 0 && splits >= limit-1 {
			break
		}
		matchStart := match.Index
		matchEnd := match.Index + match.Length
		if match.Length == 0 && matchStart == 0 {
			match, err = plan.findNextValid(text, match)
			if err != nil {
				return nil, err
			}
			continue
		}
		if matchStart < lastEnd {
			match, err = plan.findNextValid(text, match)
			if err != nil {
				return nil, err
			}
			continue
		}
		parts = append(parts, string(textRunes[lastEnd:matchStart]))
		lastEnd = matchEnd
		splits++
		if matchStart == len(textRunes) {
			break
		}
		match, err = plan.findNextValid(text, match)
		if err != nil {
			return nil, err
		}
	}
	parts = append(parts, string(textRunes[lastEnd:]))
	if splits == 0 {
		parts = []string{text}
	}
	if limit == 0 && splits > 0 {
		for len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
	}
	return parts, nil
}

func regexContainsNumericBackreference(source string) bool {
	inClass := false
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			if i+1 >= len(source) {
				continue
			}
			next := source[i+1]
			if !inClass && next >= '1' && next <= '9' {
				return true
			}
			i++
		case '[':
			inClass = true
		case ']':
			inClass = false
		}
	}
	return false
}
