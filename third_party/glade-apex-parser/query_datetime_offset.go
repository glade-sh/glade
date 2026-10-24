//go:build cgo

package apexast

import "time"

// The tree grammar requires a colon in numeric DateTime offsets. Mask compact
// offsets only in query literals; declaration ranges and extracted source keep
// the original bytes, which the query parser validates at execution time.
func normalizeInlineQueryOffsets(source string) string {
	var out []byte
	queryEnd := -1
	for i := 0; i < len(source); {
		switch {
		case source[i] == '\'':
			i = skipApexString(source, i)
		case hasPrefixAt(source, i, "//"):
			i = skipUntilNewline(source, i)
		case hasPrefixAt(source, i, "/*"):
			i = skipBlockComment(source, i)
		case source[i] == '[':
			start := nextSignificantIndex(source, i+1)
			if hasWordAtFold(source, start, "SELECT") || hasWordAtFold(source, start, "FIND") {
				queryEnd = inlineSOSLEnd(source, start)
			}
			i++
		case i < queryEnd && source[i] >= '0' && source[i] <= '9':
			end := i
			for end < queryEnd && ((source[end] >= '0' && source[end] <= '9') || source[end] == '-' || source[end] == '+' || source[end] == 'T' || source[end] == ':' || source[end] == '.' || source[end] == 'Z') {
				end++
			}
			literal := source[i:end]
			if len(literal) >= 24 {
				offset := end - 5
				if source[offset] == '+' || source[offset] == '-' {
					if _, err := time.Parse("2006-01-02T15:04:05Z0700", literal); err == nil {
						if out == nil {
							out = []byte(source)
						}
						copy(out[offset:end], "Z    ")
					}
				}
			}
			i = end
		default:
			i++
		}
	}
	if out == nil {
		return source
	}
	return string(out)
}

func originalCompactQueryOffsetRange(r Range, source string, lineMap LineMap) Range {
	offset := r.End.Offset - 1
	end := offset + len("+0000")
	if offset < r.Start.Offset || end > len(source) || (source[offset] != '+' && source[offset] != '-') {
		return r
	}
	if _, err := time.Parse("2006-01-02T15:04:05Z0700", source[r.Start.Offset:end]); err == nil {
		r.End = lineMap.Position(end)
	}
	return r
}
