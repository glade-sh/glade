package vm

import "strings"

// Inline query timestamps are one token. The caller restricts recognition to
// bracketed queries; the query parser validates zones and literal spelling.
func queryDateTimeLiteralEnd(source string, start int) int {
	if len(source)-start < 19 || source[start+4] != '-' || source[start+7] != '-' ||
		source[start+10] != 'T' || source[start+13] != ':' || source[start+16] != ':' {
		return start
	}
	for _, offset := range []int{0, 1, 2, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17, 18} {
		if source[start+offset] < '0' || source[start+offset] > '9' {
			return start
		}
	}
	end := start + 19
	for end < len(source) && strings.ContainsRune("0123456789.Z:+-", rune(source[end])) {
		end++
	}
	return end
}
