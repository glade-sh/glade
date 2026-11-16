//go:build cgo

package apexast

// The tree grammar predates the SET OPTIONS clause (API 68 queryoptions
// capture). normalizeInlineSOQLFormula blanks the trailing clause of an
// inline query so the tree parses; query validation and execution read the
// original source text.

// inlineSOQLSetOptionsStart returns the start of a top-level
// "SET OPTIONS :bind" that ends the query at end, or -1.
func inlineSOQLSetOptionsStart(source string, start, end int) int {
	depth := 0
	for i := start; i < end; {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
			continue
		case source[i] == '(':
			depth++
		case source[i] == ')':
			depth--
		case depth == 0 && isIdentifierByte(source[i]) && (i == start || !isIdentifierByte(source[i-1])) && hasWordAtFold(source, i, "SET"):
			options := nextSignificantIndex(source, i+len("SET"))
			if options < end && hasWordAtFold(source, options, "OPTIONS") {
				colon := nextSignificantIndex(source, options+len("OPTIONS"))
				if colon < end && source[colon] == ':' {
					name := nextSignificantIndex(source, colon+1)
					j := name
					for j < end && (isIdentifierByte(source[j]) || source[j] == '.') {
						j++
					}
					if j > name && nextSignificantIndex(source, j) >= end {
						return i
					}
				}
			}
		}
		i++
	}
	return -1
}
