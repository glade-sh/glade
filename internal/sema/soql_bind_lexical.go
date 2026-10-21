package sema

import (
	"regexp"
	"sort"
	"strings"
)

const semaBindingDeclarationPrefix = `(?im)(?:^|[;({,}])\s*(?:@[A-Za-z_][A-Za-z0-9_]*(?:\([^)]*\))?\s*)*(?:(?:public|private|protected|global|static|final|transient|virtual|abstract|override|webservice)\s+)*`

var semaCompactGenericBindingDeclaration = regexp.MustCompile(semaBindingDeclarationPrefix + `([A-Za-z_][A-Za-z0-9_]*(?:\s*\.\s*[A-Za-z_][A-Za-z0-9_]*)*\s*<[^;=(){}]+?>(?:\[\])?)\s*([A-Za-z_][A-Za-z0-9_]*)\b`)

var semaBindingStaticModifier = regexp.MustCompile(`(?i)\bstatic\b`)

func semaBindingStaticHeader(header string) bool {
	return semaBindingStaticModifier.MatchString(header)
}

// maskSemaBindingText excludes comments and strings without moving source
// positions or line boundaries. Bind discovery must not interpret their text.
func maskSemaBindingText(source string, spans semaCodeSpans) string {
	out := []byte(source)
	for i := range out {
		if !spans.contains(i) && out[i] != '\n' && out[i] != '\r' {
			out[i] = ' '
		}
	}
	return string(out)
}

// semaBindingDeclarationMatches keeps the type and name spans used by the
// existing resolver. A declaration's later comma-separated names share its
// type, but each retains its own visibility offset.
func semaBindingDeclarationMatches(source string) [][]int {
	matches := semaBindingDeclaration.FindAllStringSubmatchIndex(source, -1)
	matches = append(matches, semaCompactGenericBindingDeclaration.FindAllStringSubmatchIndex(source, -1)...)
	sort.SliceStable(matches, func(i, j int) bool { return matches[i][4] < matches[j][4] })
	seen := make(map[int]bool)
	var out [][]int
	for _, match := range matches {
		if seen[match[4]] || !semaBindingDeclarationType(source[match[2]:match[3]]) {
			continue
		}
		seen[match[4]] = true
		out = append(out, match)
		out = append(out, semaBindingSiblingDeclarations(source, match)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i][4] < out[j][4] })
	return out
}

func semaBindingDeclarationType(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "return", "throw", "new", "else", "break", "continue", "when":
		return false
	default:
		// Case is an SObject type (DML T005/T006), not a switch label.
		return true
	}
}

func semaBindingSiblingDeclarations(source string, match []int) [][]int {
	var siblings [][]int
	i := match[5]
	for i < len(source) && strings.ContainsRune(" \t\n\r", rune(source[i])) {
		i++
	}
	if i == len(source) || source[i] == '(' {
		return nil // Method names are not multi-declarator field statements.
	}
	var depth int
	for ; i < len(source); i++ {
		switch source[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return siblings
			}
			depth--
		case '<':
			if end := semaBindingInitializerGenericEnd(source, i); end > i {
				i = end
			}
		case ';':
			if depth == 0 {
				return siblings
			}
		case ',':
			if depth != 0 {
				continue
			}
			start := i + 1
			for start < len(source) && strings.ContainsRune(" \t\n\r", rune(source[start])) {
				start++
			}
			if start == len(source) || !semaIdentifierByte(source[start]) || source[start] >= '0' && source[start] <= '9' {
				return siblings
			}
			end := start + 1
			for end < len(source) && semaIdentifierByte(source[end]) {
				end++
			}
			next := end
			for next < len(source) && strings.ContainsRune(" \t\n\r", rune(source[next])) {
				next++
			}
			if next == len(source) || !strings.ContainsRune("=,;", rune(source[next])) {
				return siblings // A typed parameter or expression, not a sibling name.
			}
			siblings = append(siblings, []int{match[0], end, match[2], match[3], start, end})
			i = end - 1
		}
	}
	return siblings
}

// A constructor's generic type arguments can contain commas at statement
// depth. Skip those groups without treating comparison operators as types.
func semaBindingInitializerGenericEnd(source string, start int) int {
	depth := 0
	for i := start; i < len(source); i++ {
		switch source[i] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				end := i
				for i+1 < len(source) && strings.ContainsRune(" \t\n\r", rune(source[i+1])) {
					i++
				}
				if i+1 < len(source) && strings.ContainsRune("({", rune(source[i+1])) {
					return end
				}
				return start
			}
		default:
			if !semaIdentifierByte(source[i]) && !strings.ContainsRune(".,[] \t\n\r", rune(source[i])) {
				return start
			}
		}
	}
	return start
}
