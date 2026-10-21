package soql

import "strings"

// scanSOQLBindFamilyExpression keeps captured member, indexed, and conditional
// Apex operands together. Query grouping punctuation is left to the SOQL parser.
func scanSOQLBindFamilyExpression(text string, start int) (int, bool) {
	if start >= len(text) {
		return start, false
	}
	end := start
	if text[end] == '(' {
		var ok bool
		end, ok = scanSOQLBindFamilyBalanced(text, end)
		if !ok {
			return start, false
		}
	} else {
		if !soqlBindIdentifierStart(text[end]) {
			return start, false
		}
		end++
		for end < len(text) && soqlBindIdentifierPart(text[end]) {
			end++
		}
	}
	for {
		next := end
		for next < len(text) && strings.ContainsRune(" \t\r\n", rune(text[next])) {
			next++
		}
		if next >= len(text) {
			break
		}
		if text[next] == '[' || text[next] == '(' {
			tail, ok := scanSOQLBindFamilyBalanced(text, next)
			if !ok {
				break
			}
			end = tail
			continue
		}
		member := next
		if text[next] == '?' {
			if next+1 < len(text) && text[next+1] == '.' {
				member = next + 2
			} else if tail, ok := scanSOQLBindTernaryTail(text, next); ok {
				end = tail
				continue
			} else {
				break
			}
		} else if text[next] == '.' {
			member++
		} else {
			break
		}
		for member < len(text) && strings.ContainsRune(" \t\r\n", rune(text[member])) {
			member++
		}
		if member >= len(text) || !soqlBindIdentifierStart(text[member]) {
			break
		}
		member++
		for member < len(text) && soqlBindIdentifierPart(text[member]) {
			member++
		}
		end = member
	}
	return end, true
}

func scanSOQLBindTernaryTail(text string, question int) (int, bool) {
	trueStart := question + 1
	for trueStart < len(text) && strings.ContainsRune(" \t\r\n", rune(text[trueStart])) {
		trueStart++
	}
	trueEnd, ok := scanSOQLBindConditionalOperand(text, trueStart)
	if !ok {
		return question, false
	}
	trueEnd = scanSOQLBindAdditiveTail(text, trueEnd)
	colon := trueEnd
	for colon < len(text) && strings.ContainsRune(" \t\r\n", rune(text[colon])) {
		colon++
	}
	if colon >= len(text) || text[colon] != ':' {
		return question, false
	}
	falseStart := colon + 1
	for falseStart < len(text) && strings.ContainsRune(" \t\r\n", rune(text[falseStart])) {
		falseStart++
	}
	falseEnd, ok := scanSOQLBindConditionalOperand(text, falseStart)
	if !ok {
		return question, false
	}
	return scanSOQLBindAdditiveTail(text, falseEnd), true
}

func scanSOQLBindConditionalOperand(text string, start int) (int, bool) {
	if start < len(text) && text[start] == '(' {
		return scanSOQLBindFamilyExpression(text, start)
	}
	return scanSOQLBindAdditiveOperand(text, start)
}

func scanSOQLBindFamilyBalanced(text string, start int) (int, bool) {
	close := byte(')')
	if text[start] == '[' {
		close = ']'
	}
	stack := []byte{close}
	for i := start + 1; i < len(text); i++ {
		if text[i] == '\'' {
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == '\'' {
					if i+1 < len(text) && text[i+1] == '\'' {
						i += 2
						continue
					}
					break
				}
				i++
			}
			if i >= len(text) {
				return start, false
			}
			continue
		}
		switch text[i] {
		case '(':
			stack = append(stack, ')')
		case '[':
			stack = append(stack, ']')
		case ')', ']':
			if text[i] != stack[len(stack)-1] {
				return start, false
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, true
			}
		}
	}
	return start, false
}

// BindExpression retains the original query offset of an Apex bind operand.
type BindExpression struct {
	Text   string
	Offset int
}

// BindExpressions shares quote/comment handling and operand boundaries with
// the query parser. Date-literal numeric suffixes are not Apex expressions.
func BindExpressions(query string) ([]BindExpression, error) {
	tokens, err := lex(query)
	if err != nil {
		return nil, err
	}
	var out []BindExpression
	for _, tok := range tokens {
		if !strings.HasPrefix(tok.text, ":") || len(tok.text) < 2 {
			continue
		}
		text := strings.TrimSpace(tok.text[1:])
		if text == "" || text[0] != '(' && !soqlBindIdentifierStart(text[0]) {
			continue
		}
		out = append(out, BindExpression{Text: text, Offset: tok.bindOffset})
	}
	return out, nil
}
