package visualforce

import (
	"fmt"
	"strconv"
	"strings"
)

// protectNativeEntities rewrites HTML entities in attribute values and inside
// {!...} expressions so that the HTML tokenizer, which decodes every entity,
// yields the text Visualforce keeps. Natively an attribute value or an
// expression is not entity-decoded: a known HTML 4 name becomes its decimal
// reference as text ("&amp;" is "&#38;"), and everything else, including
// numeric references, other names and a bare ampersand, stays as typed. The
// value is then escaped on output, so value="a &amp; b" renders a &amp;#38; b.
// Markup text outside expressions keeps normal entity decoding.
//
// The result feeds only the tokenizer. Source positions and raw attributes
// come from the original source.
func protectNativeEntities(source string) string {
	if !strings.Contains(source, "&") {
		return source
	}
	var out strings.Builder
	copied := 0
	changed := false
	flush := func(upto int) {
		out.WriteString(source[copied:upto])
		copied = upto
	}
	replace := func(start, end int) {
		protected := protectEntityText(source[start:end])
		if protected == source[start:end] {
			return
		}
		flush(start)
		out.WriteString(protected)
		copied = end
		changed = true
	}
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "<!--") {
			end := strings.Index(source[i:], "-->")
			if end < 0 {
				break
			}
			i += end + 3
			continue
		}
		if source[i] == '<' && i+1 < len(source) && isASCIILetter(source[i+1]) {
			nameStart := i + 1
			i = nameStart
			for i < len(source) && isTagNameByte(source[i]) {
				i++
			}
			name := strings.ToLower(source[nameStart:i])
			for i < len(source) && source[i] != '>' {
				quote := source[i]
				if quote != '"' && quote != '\'' {
					i++
					continue
				}
				i++
				valueStart := i
				for i < len(source) && source[i] != quote {
					if quote == '"' && strings.HasPrefix(source[i:], "{!") {
						if end := findExpressionTemplateEnd(source, i+2); end > 0 && !strings.Contains(source[i:end], "<") {
							i = end + 1
							continue
						}
					}
					i++
				}
				replace(valueStart, i)
				if i < len(source) {
					i++
				}
			}
			if i < len(source) {
				i++
			}
			if name == "script" || name == "style" {
				if closeIdx := indexFoldASCII(source[i:], "</"+name); closeIdx >= 0 {
					i += closeIdx
				} else {
					break
				}
			}
			continue
		}
		if strings.HasPrefix(source[i:], "{!") {
			if end := findExpressionTemplateEnd(source, i+2); end > 0 && !containsTagStart(source[i:end]) {
				replace(i, end+1)
				i = end + 1
				continue
			}
		}
		if source[i] == '&' {
			// Markup text decodes with the HTML 4 table; Go's tokenizer maps
			// these two names to the HTML 5 angle brackets instead.
			if reference, length := textAngleEntity(source[i:]); length > 0 {
				flush(i)
				out.WriteString(reference)
				i += length
				copied = i
				changed = true
				continue
			}
		}
		i++
	}
	if !changed {
		return source
	}
	flush(len(source))
	return out.String()
}

func containsTagStart(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '<' && (isASCIILetter(s[i+1]) || s[i+1] == '/' || s[i+1] == '!') {
			return true
		}
	}
	return false
}

// protectEntityText returns text with every ampersand written as &amp; for the
// tokenizer, and each known HTML 4 name written as its decimal reference.
func protectEntityText(text string) string {
	if !strings.Contains(text, "&") {
		return text
	}
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '&' {
			out.WriteByte(text[i])
			continue
		}
		out.WriteString("&amp;")
		end := i + 1
		for end < len(text) && isEntityNameByte(text[end]) {
			end++
		}
		if end > i+1 && end < len(text) && text[end] == ';' {
			if code, ok := nativeAttributeEntities[text[i+1:end]]; ok {
				out.WriteString("#" + strconv.Itoa(code) + ";")
				i = end
			}
		}
	}
	return out.String()
}

func isEntityNameByte(b byte) bool {
	return isASCIILetter(b) || b >= '0' && b <= '9'
}

func textAngleEntity(s string) (string, int) {
	switch {
	case strings.HasPrefix(s, "&lang;"):
		return "&#9001;", len("&lang;")
	case strings.HasPrefix(s, "&rang;"):
		return "&#9002;", len("&rang;")
	}
	return "", 0
}

// textEntityError reports the compile error Visualforce gives for a malformed
// or undeclared entity in markup text. Text is read as XML with the HTML 4
// names declared: an unknown name, a name without its semicolon, a bare
// ampersand and a bad character reference are errors. Attribute values and
// {!...} expressions are not entity-decoded, so they never report one.
func textEntityError(source string) error {
	if !strings.Contains(source, "&") {
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
			nameStart := i + 1
			i = nameStart
			for i < len(source) && isTagNameByte(source[i]) {
				i++
			}
			name := strings.ToLower(source[nameStart:i])
			for i < len(source) && source[i] != '>' {
				quote := source[i]
				i++
				if quote != '"' && quote != '\'' {
					continue
				}
				for i < len(source) && source[i] != quote {
					if quote == '"' && strings.HasPrefix(source[i:], "{!") {
						if end := findExpressionTemplateEnd(source, i+2); end > 0 && !strings.Contains(source[i:end], "<") {
							i = end + 1
							continue
						}
					}
					i++
				}
				if i < len(source) {
					i++
				}
			}
			if i < len(source) {
				i++
			}
			if name == "script" || name == "style" {
				if closeIdx := indexFoldASCII(source[i:], "</"+name); closeIdx >= 0 {
					i += closeIdx
				} else {
					return nil
				}
			}
			continue
		}
		if strings.HasPrefix(source[i:], "{!") {
			if end := findExpressionTemplateEnd(source, i+2); end > 0 && !containsTagStart(source[i:end]) {
				i = end + 1
				continue
			}
		}
		if source[i] == '&' {
			next, err := scanTextReference(source, i)
			if err != nil {
				return err
			}
			i = next
			continue
		}
		i++
	}
	return nil
}

// scanTextReference checks the reference that starts at source[at] == '&' and
// returns the offset after it.
func scanTextReference(source string, at int) (int, error) {
	i := at + 1
	if i < len(source) && source[i] == '#' {
		i++
		hex := i < len(source) && source[i] == 'x'
		if hex {
			i++
		}
		digits := i
		for i < len(source) && (source[i] >= '0' && source[i] <= '9' || hex && (source[i] >= 'a' && source[i] <= 'f' || source[i] >= 'A' && source[i] <= 'F')) {
			i++
		}
		if i == digits {
			if hex {
				return 0, fmt.Errorf(`A hexadecimal representation must immediately follow the "&#x" in a character reference.`)
			}
			return 0, fmt.Errorf(`A decimal representation must immediately follow the "&#" in a character reference.`)
		}
		if i >= len(source) || source[i] != ';' {
			return 0, fmt.Errorf("The character reference must end with the ';' delimiter.")
		}
		return i + 1, nil
	}
	nameStart := i
	for i < len(source) && (isEntityNameByte(source[i]) || source[i] == '_' || source[i] == '-' || source[i] == '.' || source[i] == ':') {
		i++
	}
	if i == nameStart {
		return 0, fmt.Errorf("The entity name must immediately follow the '&' in the entity reference.")
	}
	name := source[nameStart:i]
	if i >= len(source) || source[i] != ';' {
		return 0, fmt.Errorf("The reference to entity %q must end with the ';' delimiter.", name)
	}
	if _, ok := nativeAttributeEntities[name]; !ok {
		return 0, fmt.Errorf("The entity %q was referenced, but not declared.", name)
	}
	return i + 1, nil
}

// indexFoldASCII returns the index of the first ASCII case-insensitive match of
// lower (already lower case) in s, or -1. It compares in place, so a scan per
// script or style tag does not copy the rest of the source.
func indexFoldASCII(s, lower string) int {
	n := len(lower)
	for i := 0; i+n <= len(s); i++ {
		if s[i] != '<' {
			continue
		}
		match := true
		for j := 0; j < n; j++ {
			c := s[i+j]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != lower[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
