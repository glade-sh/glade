package visualforce

import (
	"fmt"
	"strings"
)

// normalizeExpressionQuotes rewrites double quotes that sit inside a {!...}
// expression within a double-quoted attribute value to &quot;. Visualforce
// accepts value="{!["1"="1"]}"; an HTML or XML tokenizer would end the value at
// the first inner quote. Quotes outside expressions, markup text, comments and
// single-quoted values are left alone, and an expression that does not close
// before the next '<' is left alone so malformed source keeps its diagnostics.
func normalizeExpressionQuotes(source string) string {
	if !strings.Contains(source, `{!`) {
		return source
	}
	var out strings.Builder
	changed := false
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "<!--") {
			end := strings.Index(source[i:], "-->")
			if end < 0 {
				out.WriteString(source[i:])
				break
			}
			out.WriteString(source[i : i+end+3])
			i += end + 3
			continue
		}
		if source[i] != '<' || i+1 >= len(source) || !isASCIILetter(source[i+1]) {
			out.WriteByte(source[i])
			i++
			continue
		}
		// Inside a start tag until its closing '>'.
		out.WriteByte(source[i])
		i++
		for i < len(source) && source[i] != '>' {
			switch source[i] {
			case '\'':
				end := strings.IndexByte(source[i+1:], '\'')
				if end < 0 {
					out.WriteString(source[i:])
					i = len(source)
					continue
				}
				out.WriteString(source[i : i+end+2])
				i += end + 2
			case '"':
				out.WriteByte('"')
				i++
				for i < len(source) && source[i] != '"' {
					if strings.HasPrefix(source[i:], "{!") {
						if end := findExpressionTemplateEnd(source, i+2); end > 0 && !strings.Contains(source[i:end], "<") {
							segment := source[i : end+1]
							if strings.Contains(segment, `"`) {
								changed = true
								segment = strings.ReplaceAll(segment, `"`, "&quot;")
							}
							out.WriteString(segment)
							i = end + 1
							continue
						}
					}
					out.WriteByte(source[i])
					i++
				}
				if i < len(source) {
					out.WriteByte('"')
					i++
				}
			default:
				out.WriteByte(source[i])
				i++
			}
		}
	}
	if !changed {
		return source
	}
	return out.String()
}

func isASCIILetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

// entityInExpressionError reports the native compile error for an HTML entity
// that sits inside a {!...} expression outside a string literal. Natively the
// expression text is not entity-decoded, so &quot; where an operand is expected
// fails with "Found '&'", and an entity after a complete operand (the formula
// concatenation operator '&' followed by a name and ';') fails as a plain
// syntax error. Entities inside string literals, in attribute values outside an
// expression, and in markup text outside an expression are accepted. It scans
// the original source, before normalizeExpressionQuotes rewrites inner quotes.
func entityInExpressionError(source string) error {
	if !strings.Contains(source, "{!") || !strings.Contains(source, "&") {
		return nil
	}
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "<!--") {
			end := strings.Index(source[i:], "-->")
			if end < 0 {
				return nil
			}
			i += end + 3
			continue
		}
		if source[i] == '<' && i+1 < len(source) && isASCIILetter(source[i+1]) {
			i++
			for i < len(source) && source[i] != '>' {
				if source[i] != '"' && source[i] != '\'' {
					i++
					continue
				}
				quote := source[i]
				i++
				for i < len(source) && source[i] != quote {
					if strings.HasPrefix(source[i:], "{!") {
						next, err := scanEntityExpression(source, i)
						if err != nil {
							return err
						}
						if next > i {
							i = next
							continue
						}
					}
					i++
				}
				if i < len(source) {
					i++
				}
			}
			continue
		}
		if strings.HasPrefix(source[i:], "{!") {
			next, err := scanEntityExpression(source, i)
			if err != nil {
				return err
			}
			if next > i {
				i = next
				continue
			}
		}
		i++
	}
	return nil
}

// scanEntityExpression checks the expression that starts at source[start] ("{!")
// and returns the index after it, or start when it does not close before the
// next '<' (malformed source keeps its other diagnostics).
func scanEntityExpression(source string, start int) (int, error) {
	end := findExpressionTemplateEnd(source, start+2)
	if end < 0 || strings.Contains(source[start:end], "<") {
		return start, nil
	}
	var quote byte
	escaped := false
	previous := byte(0)
	for i := start + 2; i < end; i++ {
		c := source[i]
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == quote:
				quote = 0
				previous = c
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '&' && expressionEntityAt(source[i:end]) {
			if previous == 0 || strings.IndexByte("(,+-*/^=<>!&|?:[", previous) >= 0 {
				return 0, fmt.Errorf("Syntax error.  Found '&'")
			}
			return 0, fmt.Errorf("Syntax error")
		}
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			previous = c
		}
	}
	return end + 1, nil
}

// expressionEntityAt reports whether s starts with a named or numeric entity.
func expressionEntityAt(s string) bool {
	semi := strings.IndexByte(s, ';')
	if semi < 2 || semi > 32 {
		return false
	}
	name := s[1:semi]
	if name[0] == '#' {
		digits := name[1:]
		base := 10
		if len(digits) > 0 && (digits[0] == 'x' || digits[0] == 'X') {
			digits, base = digits[1:], 16
		}
		if digits == "" {
			return false
		}
		for i := 0; i < len(digits); i++ {
			c := digits[i]
			if !(c >= '0' && c <= '9' || base == 16 && (c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return true
	}
	if !isASCIILetter(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isASCIILetter(name[i]) && !(name[i] >= '0' && name[i] <= '9') {
			return false
		}
	}
	return true
}
