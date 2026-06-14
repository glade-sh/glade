package lightningout

import "strings"

type UseCall struct {
	App string
}

type CreateCall struct {
	Component string
	Locator   string // element id or selector
}

type Calls struct {
	Use    []UseCall
	Create []CreateCall
}

func ParseLightningCalls(script string) (Calls, error) {
	var calls Calls
	for _, args := range findLightningCallArgs(script, "$Lightning.use") {
		parts := splitTopLevelArgs(args)
		if len(parts) == 0 {
			continue
		}
		app, ok := jsStringLiteral(parts[0])
		if !ok {
			continue
		}
		calls.Use = append(calls.Use, UseCall{App: app})
	}
	for _, args := range findLightningCallArgs(script, "$Lightning.createComponent") {
		parts := splitTopLevelArgs(args)
		if len(parts) < 3 {
			continue
		}
		component, ok := jsStringLiteral(parts[0])
		if !ok {
			continue
		}
		locator, ok := jsStringLiteral(parts[2])
		if !ok {
			continue
		}
		calls.Create = append(calls.Create, CreateCall{
			Component: component,
			Locator:   locator,
		})
	}
	return calls, nil
}

func findLightningCallArgs(script, callee string) []string {
	var out []string
	state := jsScanState{}
	for i := 0; i < len(script); i++ {
		if state.consume(script, i) {
			continue
		}
		if state.active() {
			continue
		}
		switch script[i] {
		case '\'', '"', '`':
			state.quote = script[i]
			continue
		case '/':
			if i+1 < len(script) && script[i+1] == '/' {
				state.lineComment = true
				i++
				continue
			}
			if i+1 < len(script) && script[i+1] == '*' {
				state.blockComment = true
				i++
				continue
			}
		}
		if !strings.HasPrefix(script[i:], callee) {
			continue
		}
		open := i + len(callee)
		for open < len(script) && isJSWhitespace(script[open]) {
			open++
		}
		if open >= len(script) || script[open] != '(' {
			continue
		}
		args, end, ok := scanCallArguments(script, open)
		if !ok {
			i = open
			continue
		}
		out = append(out, args)
		i = end - 1
	}
	return out
}

func scanCallArguments(source string, open int) (string, int, bool) {
	if open < 0 || open >= len(source) || source[open] != '(' {
		return "", open, false
	}
	depth := 1
	state := jsScanState{}
	for i := open + 1; i < len(source); i++ {
		if state.consume(source, i) {
			continue
		}
		if state.active() {
			continue
		}
		switch source[i] {
		case '\'', '"', '`':
			state.quote = source[i]
		case '/':
			if i+1 < len(source) && source[i+1] == '/' {
				state.lineComment = true
				i++
			} else if i+1 < len(source) && source[i+1] == '*' {
				state.blockComment = true
				i++
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return source[open+1 : i], i + 1, true
			}
		}
	}
	return "", len(source), false
}

func splitTopLevelArgs(args string) []string {
	var out []string
	start := 0
	parenDepth := 0
	braceDepth := 0
	bracketDepth := 0
	state := jsScanState{}
	for i := 0; i < len(args); i++ {
		if state.consume(args, i) {
			continue
		}
		if state.active() {
			continue
		}
		switch args[i] {
		case '\'', '"', '`':
			state.quote = args[i]
		case '/':
			if i+1 < len(args) && args[i+1] == '/' {
				state.lineComment = true
				i++
			} else if i+1 < len(args) && args[i+1] == '*' {
				state.blockComment = true
				i++
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case ',':
			if parenDepth == 0 && braceDepth == 0 && bracketDepth == 0 {
				out = append(out, strings.TrimSpace(args[start:i]))
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(args[start:]) != "" || len(out) > 0 {
		out = append(out, strings.TrimSpace(args[start:]))
	}
	return out
}

type jsScanState struct {
	quote        byte
	escaped      bool
	lineComment  bool
	blockComment bool
}

func (s *jsScanState) active() bool {
	return s.quote != 0 || s.lineComment || s.blockComment
}

func (s *jsScanState) consume(source string, i int) bool {
	if s.quote != 0 {
		if s.escaped {
			s.escaped = false
			return true
		}
		switch source[i] {
		case '\\':
			s.escaped = true
		case s.quote:
			s.quote = 0
		}
		return true
	}
	if s.lineComment {
		if source[i] == '\n' || source[i] == '\r' {
			s.lineComment = false
		}
		return true
	}
	if s.blockComment {
		if source[i] == '/' && i > 0 && source[i-1] == '*' {
			s.blockComment = false
		}
		return true
	}
	return false
}

func jsStringLiteral(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return "", false
	}
	quote := raw[0]
	if quote != '\'' && quote != '"' {
		return "", false
	}
	if raw[len(raw)-1] != quote {
		return "", false
	}
	body := raw[1 : len(raw)-1]
	if !strings.Contains(body, `\`) {
		return body, true
	}
	var b strings.Builder
	escaped := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		b.WriteByte(ch)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String(), true
}

func isJSWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}
