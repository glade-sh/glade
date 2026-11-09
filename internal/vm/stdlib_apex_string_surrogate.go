package vm

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Apex strings are UTF-16: substring, left, right, mid and abbreviate can cut
// inside a surrogate pair and leave a lone surrogate that length, getChars and
// the other members then count as one unit (stdlib_surrogate_c14). Glade keeps
// strings as Go strings, so a lone surrogate U+D800..U+DFFF is stored in the
// generalized three-byte UTF-8 form ED A0..BF 80..BF, which utf8 refuses to
// encode and decodes as three RuneError bytes. The helpers here decode and
// encode that form so the UTF-16 index model stays exact; every other string
// operation keeps working on the bytes.

const loneSurrogateLead = 0xED

// mayHoldLoneSurrogate is the hot-path gate: text without an ED lead byte has
// no encoded lone surrogate, so the plain range loops apply. Hangul syllables
// (ED 80..9F) also start with ED and take the exact loops, which stay correct.
func mayHoldLoneSurrogate(text string) bool {
	return strings.IndexByte(text, loneSurrogateLead) >= 0
}

// isLoneSurrogateAt reports whether text[i:] starts with an encoded lone
// surrogate. Valid UTF-8 never has ED followed by A0..BF (that range is the
// surrogates themselves), so the check cannot misread Hangul (ED 80..9F).
func isLoneSurrogateAt(text string, i int) bool {
	return len(text)-i >= 3 && text[i] == loneSurrogateLead &&
		text[i+1] >= 0xA0 && text[i+1] <= 0xBF && text[i+2] >= 0x80 && text[i+2] <= 0xBF
}

// decodeApexRune is utf8.DecodeRuneInString that also decodes an encoded lone
// surrogate as its code unit with size 3.
func decodeApexRune(text string) (rune, int) {
	r, size := utf8.DecodeRuneInString(text)
	if r == utf8.RuneError && size == 1 && isLoneSurrogateAt(text, 0) {
		return rune(text[1]&0x3F)<<6 | rune(text[2]&0x3F) | 0xD000, 3
	}
	return r, size
}

// apexRuneWidth is the UTF-16 width of a decoded rune: two units above the BMP.
func apexRuneWidth(r rune) int {
	if r > 0xffff {
		return 2
	}
	return 1
}

// appendApexUnit appends one UTF-16 code unit: a BMP character as UTF-8, a
// surrogate in the lone three-byte form.
func appendApexUnit(b []byte, unit uint16) []byte {
	if unit >= 0xD800 && unit <= 0xDFFF {
		return append(b, loneSurrogateLead, 0x80|byte(unit>>6)&0x3F, 0x80|byte(unit)&0x3F)
	}
	return utf8.AppendRune(b, rune(unit))
}

// apexStringFromUTF16Units encodes units as Apex text: valid pairs become one
// code point and lone surrogates keep their three-byte form, as native's
// String.fromCharArray does.
func apexStringFromUTF16Units(units []uint16) string {
	b := make([]byte, 0, len(units)+len(units)/2)
	for i := 0; i < len(units); i++ {
		unit := units[i]
		if unit >= 0xD800 && unit <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
			b = utf8.AppendRune(b, (rune(unit)-0xD800)<<10|(rune(units[i+1])-0xDC00)+0x10000)
			i++
			continue
		}
		b = appendApexUnit(b, unit)
	}
	return string(b)
}

// hasLoneSurrogate reports whether text holds an encoded lone surrogate.
func hasLoneSurrogate(text string) bool {
	for i := 0; i < len(text); {
		next := strings.IndexByte(text[i:], loneSurrogateLead)
		if next < 0 {
			return false
		}
		i += next
		if isLoneSurrogateAt(text, i) {
			return true
		}
		i++
	}
	return false
}

// joinApexStrings concatenates two Apex strings. A lone high surrogate ending
// left and a lone low surrogate starting right form one code point, as they
// do in native's UTF-16 arrays ('rejoin', 'lone_lo_concat').
func joinApexStrings(left, right string) string {
	if len(left) >= 3 && len(right) >= 3 && left[len(left)-3] == loneSurrogateLead && right[0] == loneSurrogateLead &&
		isLoneSurrogateAt(left, len(left)-3) && left[len(left)-2] <= 0xAF && isLoneSurrogateAt(right, 0) && right[1] >= 0xB0 {
		hi, _ := decodeApexRune(left[len(left)-3:])
		lo, _ := decodeApexRune(right)
		b := make([]byte, 0, len(left)+len(right)-2)
		b = append(b, left[:len(left)-3]...)
		b = utf8.AppendRune(b, (hi-0xD800)<<10|(lo-0xDC00)+0x10000)
		b = append(b, right[3:]...)
		return string(b)
	}
	return left + right
}

// apexSubstringUnits returns the UTF-16 units [start, end) of text. Callers
// validate the range. A boundary inside a surrogate pair yields that pair's
// lone unit, as native substring does; whole-rune ranges slice without copying.
func apexSubstringUnits(text string, start, end int) string {
	if start >= end {
		return ""
	}
	if !mayHoldLoneSurrogate(text) {
		if startByte, ok := apexUnitBoundary(text, start); ok {
			if endByte, ok := apexUnitBoundary(text, end); ok {
				return text[startByte:endByte]
			}
		}
	}
	return apexSubstringUnitsExact(text, start, end)
}

// apexUnitBoundary is the byte offset of UTF-16 unit index in text free of
// encoded lone surrogates; ok is false when the index splits a surrogate pair,
// the case apexSubstringUnitsExact settles.
func apexUnitBoundary(text string, index int) (int, bool) {
	if index <= 0 {
		return 0, true
	}
	unit := 0
	for i, r := range text {
		if unit == index {
			return i, true
		}
		if r <= 0xffff {
			unit++
			continue
		}
		if unit+1 == index {
			return 0, false
		}
		unit += 2
	}
	return len(text), true
}

func apexSubstringUnitsExact(text string, start, end int) string {
	if start == 0 && end >= len(text) {
		// end is a unit count no smaller than the byte count only when every
		// unit is one byte: the whole text.
		return text
	}
	unit, skip := 0, 0
	startByte := -1
	var prefix []byte
	for i, r := range text {
		if skip > 0 {
			skip--
			continue
		}
		if unit == start && startByte < 0 {
			startByte = i
		}
		width, size := 1, utf8.RuneLen(r)
		switch {
		case r > 0xffff:
			width = 2
		case r == utf8.RuneError && isLoneSurrogateAt(text, i):
			size, skip = 3, 2
		}
		if width == 2 {
			switch {
			case unit+1 == start:
				// The range starts on the low surrogate of this pair.
				_, lo := surrogatePairUnits(r)
				prefix = appendApexUnit(nil, lo)
				if unit+2 >= end {
					return string(prefix)
				}
				startByte = i + size
			case unit+1 == end:
				// The range ends on the high surrogate of this pair.
				hi, _ := surrogatePairUnits(r)
				return string(appendApexUnit(append(prefix, text[startByte:i]...), hi))
			}
		}
		unit += width
		if unit >= end {
			if prefix == nil {
				return text[startByte : i+size]
			}
			return string(append(prefix, text[startByte:i+size]...))
		}
	}
	if startByte < 0 {
		return ""
	}
	if prefix == nil {
		return text[startByte:]
	}
	return string(append(prefix, text[startByte:]...))
}

func surrogatePairUnits(r rune) (uint16, uint16) {
	r -= 0x10000
	return uint16(0xD800 + (r >> 10)), uint16(0xDC00 + (r & 0x3FF))
}

// apexStringUTF8Bytes is the UTF-8 encoding native produces for text leaving
// the UTF-16 world (Blob.valueOf, EncodingUtil.urlEncode): each lone
// surrogate becomes '?'.
func apexStringUTF8Bytes(text string) string {
	if !hasLoneSurrogate(text) {
		return text
	}
	b := make([]byte, 0, len(text))
	for i := 0; i < len(text); {
		if text[i] == loneSurrogateLead && isLoneSurrogateAt(text, i) {
			b = append(b, '?')
			i += 3
			continue
		}
		b = append(b, text[i])
		i++
	}
	return string(b)
}

// mapApexStringSegments applies transform to the runs of valid text between
// lone surrogates, which pass through unchanged (native case mapping leaves
// them alone). Text without a lone surrogate is transformed in one call.
func mapApexStringSegments(text string, transform func(string) string) string {
	if !hasLoneSurrogate(text) {
		return transform(text)
	}
	var b strings.Builder
	segment := 0
	for i := 0; i < len(text); {
		if text[i] == loneSurrogateLead && isLoneSurrogateAt(text, i) {
			if segment < i {
				b.WriteString(transform(text[segment:i]))
			}
			b.WriteString(text[i : i+3])
			i += 3
			segment = i
			continue
		}
		i++
	}
	if segment < len(text) {
		b.WriteString(transform(text[segment:]))
	}
	return b.String()
}

// loneNeedleIndex serves the byte-search members when the needle holds a lone
// surrogate: the bytes of a lone unit never occur inside an encoded pair, yet
// native matches code units ('s_contains_lone', 's_substringBefore_lo'). It
// reports the unit index of the first (or last) match.
func loneNeedleIndex(text, needle string, last bool) (int, bool) {
	if !hasLoneSurrogate(needle) {
		return -1, false
	}
	var at int
	if last {
		at = stringLastIndexOfUTF16(text, needle, apexStringLength(text))
	} else {
		at = stringIndexOfUTF16(text, needle, 0)
	}
	return at, at >= 0
}

func loneNeedleFound(text, needle string) bool {
	_, ok := loneNeedleIndex(text, needle, false)
	return ok
}

func loneNeedleAt(text, needle string, index int) bool {
	at, ok := loneNeedleIndex(text, needle, false)
	return ok && at == index
}

func loneNeedleAtEnd(text, needle string) bool {
	at, ok := loneNeedleIndex(text, needle, true)
	return ok && at == apexStringLength(text)-apexStringLength(needle)
}

// reverseApexString reverses the UTF-16 units and then restores each surrogate
// pair, as Java's StringBuilder.reverse does; a lone surrogate stays in place.
func reverseApexString(text string) string {
	units := apexStringUTF16Units(text)
	for i, j := 0, len(units)-1; i < j; i, j = i+1, j-1 {
		units[i], units[j] = units[j], units[i]
	}
	for i := 0; i+1 < len(units); i++ {
		if units[i] >= 0xDC00 && units[i] <= 0xDFFF && units[i+1] >= 0xD800 && units[i+1] <= 0xDBFF {
			units[i], units[i+1] = units[i+1], units[i]
			i++
		}
	}
	return apexStringFromUTF16Units(units)
}

// jsonLoneSurrogateString marshals Apex text holding a lone surrogate the way
// native JSON.serialize writes it: quoted, with the unit itself in the output
// rather than the U+FFFD that encoding/json substitutes for its bytes.
type jsonLoneSurrogateString string

func (text jsonLoneSurrogateString) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, len(text)+2)
	b = append(b, '"')
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '"' || c == '\\':
			b = append(b, '\\', c)
		case c < 0x20:
			b = append(b, []byte(fmt.Sprintf(`\u%04x`, c))...)
		default:
			b = append(b, c)
		}
	}
	return append(b, '"'), nil
}

// apexSubstringBoundary is the byte offset of UTF-16 unit index; an index inside
// a surrogate pair rounds up to the end of that pair (the char-search members
// convert unit offsets to rune counts with it).
func apexSubstringBoundary(text string, index int) int {
	if index <= 0 {
		return 0
	}
	unit := 0
	for i := 0; i < len(text); {
		if unit >= index {
			return i
		}
		if text[i] < utf8.RuneSelf {
			unit++
			i++
			continue
		}
		r, size := decodeApexRune(text[i:])
		unit += apexRuneWidth(r)
		i += size
	}
	return len(text)
}
