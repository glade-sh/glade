package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func splitSemaLeadingType(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ' ', '\t', '\r', '\n':
			if depth == 0 {
				return strings.TrimSpace(text[:i]), strings.TrimSpace(text[i+1:]), true
			}
		}
	}
	return "", "", false
}
func splitTopLevelSemaList(text string) []string {
	var out []string
	start := 0
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\'':
			i = skipSemaString(text, i)
		case '/':
			if end, ok := skipSemaComment(text, i); ok {
				i = end
			}
		case '(', '[', '{', '<':
			depth++
		case ')', ']', '}', '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	out = append(out, strings.TrimSpace(text[start:]))
	return out
}
func blockBoundsAfter(body string, pos int) (int, int) {
	for i := pos; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '{':
			return blockBoundsAt(body, i+1)
		case ';':
			return pos, pos
		}
	}
	return pos, len(body)
}
func statementOrBlockBoundsAfter(body string, pos int) (int, int) {
	for i := pos; i < len(body); i++ {
		switch body[i] {
		case ' ', '\t', '\r', '\n':
			continue
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
				continue
			}
		case '{':
			return blockBoundsAt(body, i+1)
		case ';':
			return i, i
		}
		end := semaStatementEnd(body, i)
		if end < i {
			return i, i
		}
		return i, end
	}
	return pos, len(body)
}
func (s semaScopeModel) flat() map[string]string {
	out := make(map[string]string, len(s.base)+len(s.locals))
	for name, typeName := range s.base {
		out[name] = typeName
	}
	starts := make(map[string]int, len(s.locals))
	for _, local := range s.locals {
		key := normalizeName(local.name)
		if local.start >= starts[key] {
			out[key] = local.typeName
			starts[key] = local.start
		}
	}
	return out
}
func (s semaScopeModel) flatAt(pos int) map[string]string {
	out := make(map[string]string, len(s.base)+len(s.locals))
	for name, typeName := range s.base {
		out[name] = typeName
	}
	starts := make(map[string]int, len(s.locals))
	for _, local := range s.locals {
		key := normalizeName(local.name)
		if pos >= local.start && pos <= local.scopeEnd && local.start >= starts[key] {
			out[key] = local.typeName
			starts[key] = local.start
		}
	}
	return out
}
func leadingWhitespaceLen(text string) int {
	return len(text) - len(strings.TrimLeftFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}))
}
func startsWithUpperASCII(text string) bool {
	text = strings.TrimSpace(text)
	return text != "" && text[0] >= 'A' && text[0] <= 'Z'
}
func splitSemaMethodPath(callee string) (string, string, bool) {
	depth := 0
	angleDepth := 0
	idx := -1
	for i := 0; i < len(callee); i++ {
		switch callee[i] {
		case '\'':
			i = skipSemaString(callee, i)
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case '<':
			if depth == 0 && looksLikeSemaGenericOpen(callee, i) {
				angleDepth++
			}
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case '.':
			if depth == 0 && angleDepth == 0 {
				idx = i
			}
		}
	}
	if idx <= 0 || idx >= len(callee)-1 {
		return "", "", false
	}
	return strings.TrimSpace(callee[:idx]), strings.TrimSpace(callee[idx+1:]), true
}
func splitLastSemaCall(arg string) (string, string, []semaArg, bool) {
	if !strings.HasSuffix(arg, ")") {
		return "", "", nil, false
	}
	open := matchingOpenParenBefore(arg, len(arg)-1)
	if open < 0 {
		return "", "", nil, false
	}
	methodEnd := open
	for methodEnd > 0 && isWhitespace(arg[methodEnd-1]) {
		methodEnd--
	}
	methodStart := methodEnd
	for methodStart > 0 && isIdentifierByte(arg[methodStart-1]) {
		methodStart--
	}
	dot := methodStart - 1
	for dot >= 0 && isWhitespace(arg[dot]) {
		dot--
	}
	if methodStart == methodEnd || dot < 0 || arg[dot] != '.' {
		return "", "", nil, false
	}
	receiver := strings.TrimSpace(arg[:dot])
	if receiver == "" {
		return "", "", nil, false
	}
	args, haveArgs := callArgumentsAt(arg, open)
	if !haveArgs {
		return "", "", nil, false
	}
	return receiver, strings.TrimSpace(arg[methodStart:methodEnd]), args, true
}
func splitSemaTernary(arg string) (string, string, string, bool) {
	question, colon, ok := semaTernaryPositions(arg)
	if !ok {
		return "", "", "", false
	}
	condition := strings.TrimSpace(arg[:question])
	whenTrue := strings.TrimSpace(arg[question+1 : colon])
	whenFalse := strings.TrimSpace(arg[colon+1:])
	return condition, whenTrue, whenFalse, condition != "" && whenTrue != "" && whenFalse != ""
}
func splitSemaCast(arg string) (string, string, bool) {
	if !strings.HasPrefix(arg, "(") {
		return "", "", false
	}
	close := strings.IndexByte(arg, ')')
	if close < 0 {
		return "", "", false
	}
	typeName := strings.TrimSpace(arg[1:close])
	value := strings.TrimSpace(arg[close+1:])
	if typeName == "" || value == "" {
		return "", "", false
	}
	for _, op := range []string{".", "+", "-", "*", "/", "%", "&&", "||", "==", "!=", "<=", ">=", "<", ">", "?", ":", ","} {
		if strings.HasPrefix(value, op) {
			return "", "", false
		}
	}
	if value == "" {
		return "", "", false
	}
	if !typeReferencePattern.MatchString(typeName) {
		return "", "", false
	}
	return typeName, value, true
}
func splitBareSemaCall(arg string) (string, []semaArg, bool) {
	arg = strings.TrimSpace(arg)
	if !strings.HasSuffix(arg, ")") {
		return "", nil, false
	}
	open := matchingOpenParenBefore(arg, len(arg)-1)
	if open <= 0 {
		return "", nil, false
	}
	method := strings.TrimSpace(arg[:open])
	if !simpleIdentifierPattern.MatchString(method) {
		return "", nil, false
	}
	args, haveArgs := callArgumentsAt(arg, open)
	return method, args, haveArgs
}
func matchingOpenParenBefore(body string, close int) int {
	depth := 0
	for i := close; i >= 0; i-- {
		switch body[i] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
func extractBodyForSema(source string, r diagnostic.Range) (string, int, bool) {
	start := r.Start.Offset
	end := r.End.Offset
	if start < 0 || start >= len(source) || end <= start || end > len(source) {
		return "", 0, false
	}
	text := source[start:end]
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "", 0, false
	}
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '/':
			if end, ok := skipSemaComment(text, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(text, i)
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[open+1 : i], start + open + 1, true
			}
		}
	}
	return "", 0, false
}
func skipSemaString(source string, start int) int {
	for i := start + 1; i < len(source); i++ {
		if source[i] == '\\' && i+1 < len(source) {
			i++
			continue
		}
		if source[i] == '\'' {
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				continue
			}
			return i
		}
	}
	return len(source) - 1
}
func skipSemaComment(source string, start int) (int, bool) {
	if start+1 >= len(source) || source[start] != '/' {
		return start, false
	}
	switch source[start+1] {
	case '/':
		if end := strings.IndexAny(source[start+2:], "\r\n"); end >= 0 {
			return start + 2 + end, true
		}
		return len(source) - 1, true
	case '*':
		if end := strings.Index(source[start+2:], "*/"); end >= 0 {
			return start + 2 + end + 1, true
		}
		return len(source) - 1, true
	default:
		return start, false
	}
}
func stripSemaComments(source string) string {
	var out strings.Builder
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\'':
			end := skipSemaString(source, i)
			out.WriteString(source[i : end+1])
			i = end
		case '/':
			if end, ok := skipSemaComment(source, i); ok {
				i = end
				continue
			}
			out.WriteByte(source[i])
		default:
			out.WriteByte(source[i])
		}
	}
	return out.String()
}
func blockBoundsAt(body string, pos int) (int, int) {
	start := 0
	stack := make([]int, 0)
	for i := 0; i < len(body) && i < pos; i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(body, i)
		case '{':
			stack = append(stack, i)
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) > 0 {
		start = stack[len(stack)-1]
	} else {
		return 0, len(body)
	}
	depth := 0
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '\'':
			i = skipSemaString(body, i)
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return start, i
			}
			depth--
			if depth == 0 {
				return start, i
			}
		}
	}
	return start, len(body)
}
func callArgumentsAt(body string, calleeEnd int) ([]semaArg, bool) {
	open := strings.IndexByte(body[calleeEnd:], '(')
	if open < 0 {
		return nil, false
	}
	open += calleeEnd
	parenDepth := 0
	groupDepth := 0
	angleDepth := 0
	start := open + 1
	var args []semaArg
	for i := open; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = skipSemaString(body, i)
		case '/':
			if end, ok := skipSemaComment(body, i); ok {
				i = end
			}
		case '(':
			if angleDepth == 0 && groupDepth == 0 {
				parenDepth++
			}
		case '<':
			if looksLikeSemaGenericOpen(body, i) {
				angleDepth++
			}
		case '>':
			if angleDepth > 0 && (i == 0 || body[i-1] != '=') {
				angleDepth--
			}
		case '{', '[':
			if angleDepth == 0 {
				groupDepth++
			}
		case ')':
			if angleDepth == 0 && groupDepth == 0 {
				parenDepth--
			}
			if parenDepth == 0 {
				if arg := trimSemaArg(body, start, i); arg.text != "" {
					args = append(args, arg)
				}
				return args, true
			}
		case '}', ']':
			if angleDepth == 0 && groupDepth > 0 {
				groupDepth--
			}
		case ',':
			if parenDepth == 1 && groupDepth == 0 && angleDepth == 0 {
				args = append(args, trimSemaArg(body, start, i))
				start = i + 1
			}
		}
	}
	return nil, false
}
func trimSemaArg(body string, start, end int) semaArg {
	for start < end && (body[start] == ' ' || body[start] == '\t' || body[start] == '\n' || body[start] == '\r') {
		start++
	}
	for end > start && (body[end-1] == ' ' || body[end-1] == '\t' || body[end-1] == '\n' || body[end-1] == '\r') {
		end--
	}
	return semaArg{text: strings.TrimSpace(stripSemaComments(body[start:end])), start: start, end: end}
}
func splitSemaInstanceOf(arg string) (string, string, bool) {
	depth := 0
	const op = " instanceof "
	for i := 0; i <= len(arg)-len(op); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
			continue
		case '(', '[', '{', '<':
			depth++
			continue
		case ')', ']', '}', '>':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && strings.EqualFold(arg[i:i+len(op)], op) {
			left := strings.TrimSpace(arg[:i])
			right := semaLeadingTypeToken(strings.TrimSpace(arg[i+len(op):]))
			return left, right, left != "" && right != ""
		}
	}
	return "", "", false
}
func splitSemaBinary(arg, op string) (string, string, bool) {
	depth := 0
	angleDepth := 0
	for i := 0; i <= len(arg)-len(op); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
			continue
		case '(', '[', '{':
			depth++
			continue
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			continue
		case '<':
			if depth == 0 && looksLikeSemaGenericOpen(arg, i) {
				angleDepth++
				continue
			}
		case '>':
			if angleDepth > 0 {
				angleDepth--
				continue
			}
		}
		if depth == 0 && angleDepth == 0 && strings.HasPrefix(arg[i:], op) {
			if op == "-" && strings.TrimSpace(arg[:i]) == "" {
				continue
			}
			return strings.TrimSpace(arg[:i]), strings.TrimSpace(arg[i+len(op):]), true
		}
	}
	return "", "", false
}
func looksLikeSemaGenericOpen(arg string, pos int) bool {
	left := pos - 1
	for left >= 0 && isWhitespace(arg[left]) {
		left--
	}
	right := pos + 1
	for right < len(arg) && isWhitespace(arg[right]) {
		right++
	}
	if left < 0 || right >= len(arg) || !isSemaIdentifierChar(arg[left]) || !isSemaIdentifierChar(arg[right]) {
		return false
	}
	depth := 0
	for i := pos; i < len(arg); i++ {
		switch arg[i] {
		case '\'':
			i = skipSemaString(arg, i)
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return true
			}
		case '[':
			if depth == 1 {
				next := i + 1
				for next < len(arg) && isWhitespace(arg[next]) {
					next++
				}
				if next < len(arg) && arg[next] == ']' {
					i = next
					continue
				}
				return false
			}
		case '(', '{', ';', '=':
			if depth == 1 {
				return false
			}
		}
	}
	return false
}
func skipSemaCall(callee string) bool {
	switch normalizeName(callee) {
	case "if", "for", "while", "switch", "catch", "new", "return", "throw",
		"insert", "update", "upsert", "delete", "undelete", "merge", "on", "when",
		"select", "find", "from", "where", "and", "or", "in", "not", "like", "includes", "excludes", "group", "order", "having", "limit", "offset",
		"__mapentry", "__coalesce", "__safe_call:get", "system.assert", "system.assertequals", "system.debug",
		"count", "count_distinct", "sum", "avg", "min", "max":
		return true
	default:
		return false
	}
}
func extractTypeNames(typeRef string) []string {
	matches := typeIdentifierPattern.FindAllString(typeRef, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if isCollectionType(match) {
			continue
		}
		out = append(out, match)
	}
	return out
}
