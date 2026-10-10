package vm

import (
	"fmt"
	"strings"
	"unicode"
)

// Apex follows Java Character.isWhitespace, which excludes nonbreaking spaces.
func apexStringWhitespace(r rune) bool {
	if r >= '\t' && r <= '\r' || r >= 0x1c && r <= 0x1f {
		return true
	}
	return unicode.Is(unicode.Zs, r) && r != 0xa0 && r != 0x2007 && r != 0x202f || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

func compareApexStrings(left, right string) int {
	a, b := apexStringUTF16Units(left), apexStringUTF16Units(right)
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
	}
	return len(a) - len(b)
}

func stringCodePointRangeException(text string, begin, end int) error {
	if begin < 0 || begin > apexStringLength(text) || begin > end {
		return newExceptionError("StringException", fmt.Sprintf("Starting position out of bounds: %d", begin))
	}
	return newExceptionError("StringException", fmt.Sprintf("End position out of bounds: %d", end))
}

func stringCodePointOffsetException(text string, index, offset int) error {
	if index < 0 || index > apexStringLength(text) {
		return newExceptionError("StringException", fmt.Sprintf("Starting position out of bounds: %d", index))
	}
	return newExceptionError("StringException", fmt.Sprintf("Code point offset out of bounds: %d", offset))
}

func escapeHTMLNamedEntities(text string, entities map[string]string) string {
	inverse := make(map[rune]string, len(entities)+4)
	for name, value := range entities {
		for _, r := range value {
			inverse[r] = "&" + name + ";"
		}
	}
	inverse['&'], inverse['<'], inverse['>'], inverse['"'], inverse['\''] = "&amp;", "&lt;", "&gt;", "&quot;", "&#39;"
	var b strings.Builder
	for _, r := range text {
		if entity, ok := inverse[r]; ok {
			b.WriteString(entity)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type stringReplacementGroupError struct{ group int }

func (e *stringReplacementGroupError) Error() string { return "replacement groupIndex out of range" }
