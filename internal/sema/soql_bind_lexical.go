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

// semaTextCuts maps offsets in a compacted text back to the original text. Each
// entry starts a kept segment: compacted offsets at or after start map to
// start+delta.
type semaTextCuts []semaTextCut

type semaTextCut struct {
	start int // first compacted offset of the segment
	delta int // original offset minus compacted offset within the segment
}

func (c semaTextCuts) offset(compacted int) int {
	if len(c) == 0 || compacted < 0 {
		return compacted
	}
	i := sort.Search(len(c), func(i int) bool { return c[i].start > compacted }) - 1
	if i < 0 {
		return compacted
	}
	return compacted + c[i].delta
}

// restore maps a submatch index slice in place and returns it.
func (c semaTextCuts) restore(match []int) []int {
	if len(c) == 0 {
		return match
	}
	for i, offset := range match {
		if offset >= 0 {
			match[i] = c.offset(offset)
		}
	}
	return match
}

// semaTypeScopeDeclarationText returns body with the interior of every nested
// brace block in blocks (absolute offsets; body starts at base) removed, keeping
// the braces themselves, together with the cuts that map the result back to
// body offsets. Removing an interior leaves the declaration scan's type-scope
// matches unchanged: a match cannot contain a brace except inside annotation
// parentheses, and the sibling walk only passes through a block whose brackets
// are balanced. When either condition fails for some block, the body is
// returned unchanged so the scan sees exactly what it saw before.
func semaTypeScopeDeclarationText(body string, blocks [][2]int, base int) (string, semaTextCuts) {
	if len(blocks) == 0 || semaAnnotationArgumentsCrossBrace(body) {
		return body, nil
	}
	var out []byte
	var cuts semaTextCuts
	kept := 0
	for _, block := range blocks {
		open, close := block[0]-base, block[1]-base
		if open < kept || close <= open {
			return body, nil
		}
		if close > len(body) {
			close = len(body)
		}
		if !semaSiblingWalkStaysInside(body, open+1, close) {
			return body, nil
		}
		if out == nil {
			out = make([]byte, 0, len(body)/2)
		}
		out = append(out, body[kept:open+1]...)
		cuts = append(cuts, semaTextCut{start: len(out), delta: close - len(out)})
		kept = close
	}
	out = append(out, body[kept:]...)
	return string(out), cuts
}

// semaAnnotationArgumentsCrossBrace reports whether any @Name( argument list
// reaches a brace before its closing parenthesis, the one way a declaration
// match can span a block boundary.
func semaAnnotationArgumentsCrossBrace(body string) bool {
	for i := strings.IndexByte(body, '@'); i >= 0 && i < len(body); {
		j := i + 1
		// The declaration regex is case-insensitive: its ASCII character
		// classes also match Unicode simple folds. Keep the full body when an
		// annotation name may contain one of those non-ASCII characters.
		for j < len(body) && semaIdentifierByte(body[j]) {
			j++
		}
		if j < len(body) && body[j] >= 0x80 {
			return true
		}
		j = i + 1
		if j < len(body) && semaIdentifierByte(body[j]) && !(body[j] >= '0' && body[j] <= '9') {
			for j < len(body) && semaIdentifierByte(body[j]) {
				j++
			}
			if j < len(body) && body[j] == '(' {
				for k := j + 1; k < len(body) && body[k] != ')'; k++ {
					if body[k] == '{' || body[k] == '}' {
						return true
					}
				}
			}
		}
		next := strings.IndexByte(body[i+1:], '@')
		if next < 0 {
			break
		}
		i += 1 + next
	}
	return false
}

// semaSiblingWalkStaysInside replays semaBindingSiblingDeclarations' bracket
// walk over body[start:end], the interior of one block, and reports whether the
// walk neither returns early inside it nor leaves it at a different depth. In
// that case the walk's behavior after the block's closing brace is the same
// whether or not the interior is present.
func semaSiblingWalkStaysInside(body string, start, end int) bool {
	depth := 0
	for i := start; i < end; i++ {
		switch body[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return false
			}
			depth--
		case '<':
			if skip := semaBindingInitializerGenericEnd(body, i); skip > i {
				if skip >= end {
					return false
				}
				i = skip
			}
		}
	}
	return depth == 0
}
