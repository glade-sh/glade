package vm

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// Salesforce names two Japanese charsets. Shift-JIS (with the SJIS, x-sjis and
// csShiftJIS spellings) is the JIS X 0208 encoding. Windows-31J (windows-31j,
// MS932) is code page 932, which adds the NEC row 13 and IBM extension rows and
// spells a few punctuation marks with other code points.
// NATIVE Shift-JIS and Windows-31J apex_release_gates/org.tsv anonymous 62.0 67.0

// jisPunctuationVariants pairs each JIS X 0208 punctuation mark with the code
// point that code page 932 uses at the same position. Each charset rejects the
// other charset's spelling.
var jisPunctuationVariants = map[rune]rune{
	0x2014: 0x2015, // em dash and horizontal bar
	0x301C: 0xFF5E, // wave dash and fullwidth tilde
	0x2016: 0x2225, // double vertical line and parallel to
	0x2212: 0xFF0D, // minus sign and fullwidth hyphen-minus
}

// jisCurrencyVariants pairs the Latin currency and not signs that JIS X 0208
// holds with the fullwidth forms of code page 932. Windows-31J accepts both.
var jisCurrencyVariants = map[rune]rune{
	0x00A2: 0xFFE0,
	0x00A3: 0xFFE1,
	0x00AC: 0xFFE2,
}

var cp932ToJIS = func() map[rune]rune {
	reverse := make(map[rune]rune, len(jisPunctuationVariants)+len(jisCurrencyVariants))
	for jisRune, cpRune := range jisPunctuationVariants {
		reverse[cpRune] = jisRune
	}
	for jisRune, cpRune := range jisCurrencyVariants {
		reverse[cpRune] = jisRune
	}
	return reverse
}()

// japaneseExtensionLead reports the lead bytes that code page 932 adds beyond
// JIS X 0208: NEC row 13, the NEC-selected and IBM extensions, and the
// user-defined rows.
func japaneseExtensionLead(lead byte) bool {
	return lead == 0x87 || lead == 0xED || lead == 0xEE || lead >= 0xF0
}

func encodeJapaneseURLBytes(text, charset string) []byte {
	jis := charset == "shift-jis"
	out := make([]byte, 0, len(text))
	for _, r := range text {
		out = append(out, encodeJapaneseRune(r, jis)...)
	}
	return out
}

func encodeJapaneseRune(r rune, jis bool) []byte {
	switch r {
	case 0x00A5:
		return []byte{0x5C} // the yen sign shares the backslash byte
	case 0x203E:
		return []byte{0x7E} // the overline shares the tilde byte
	}
	if jis {
		if _, cpSpelling := cp932ToJIS[r]; cpSpelling {
			return []byte{'?'}
		}
		if cpRune, ok := jisPunctuationVariants[r]; ok {
			r = cpRune
		} else if cpRune, ok := jisCurrencyVariants[r]; ok {
			r = cpRune
		}
	} else {
		if _, jisSpelling := jisPunctuationVariants[r]; jisSpelling {
			return []byte{'?'}
		}
		if cpRune, ok := jisCurrencyVariants[r]; ok {
			r = cpRune
		}
	}
	encoded, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte(string(r)))
	if err != nil || len(encoded) == 0 {
		return []byte{'?'}
	}
	if len(encoded) == 2 {
		if jis && japaneseExtensionLead(encoded[0]) {
			return []byte{'?'}
		}
		if !jis && (encoded[0] == 0xED || encoded[0] == 0xEE) {
			return necSelectedToIBM(encoded[0], encoded[1])
		}
	}
	return encoded
}

// necSelectedToIBM moves a character from the NEC-selected IBM extension rows
// to its IBM extension position, which is the code Salesforce emits. The two
// areas list the same characters in the same order.
func necSelectedToIBM(lead, trail byte) []byte {
	if lead == 0xEE && trail >= 0xEF && trail <= 0xF8 {
		return []byte{0xFA, 0x40 + (trail - 0xEF)} // small roman numerals
	}
	if lead == 0xEE && trail >= 0xFA && trail <= 0xFC {
		return []byte{0xFA, 0x55 + (trail - 0xFA)} // broken bar, apostrophe, quotation mark
	}
	trailIndex := func(trail byte) int {
		n := int(trail) - 0x40
		if trail > 0x7E {
			n--
		}
		return n
	}
	position := (int(lead)-0xED)*188 + trailIndex(trail) + 28 // FA5C is 28 past FA40
	ibmLead := byte(0xFA + position/188)
	index := position % 188
	ibmTrail := byte(0x40 + index)
	if index >= 0x3F {
		ibmTrail++
	}
	return []byte{ibmLead, ibmTrail}
}

func decodeJapaneseURLBytes(data []byte, charset string) string {
	jis := charset == "shift-jis"
	var out strings.Builder
	for i := 0; i < len(data); {
		b := data[i]
		switch {
		case b < 0x80:
			out.WriteByte(b)
			i++
		case b >= 0xA1 && b <= 0xDF:
			out.WriteRune(rune(0xFF61 + int(b) - 0xA1))
			i++
		case b >= 0x81 && b <= 0x9F || b >= 0xE0 && b <= 0xFC:
			if i+1 < len(data) && (data[i+1] >= 0x40 && data[i+1] <= 0x7E || data[i+1] >= 0x80 && data[i+1] <= 0xFC) &&
				!(jis && japaneseExtensionLead(b)) {
				if r, ok := decodeJapanesePair(b, data[i+1]); ok {
					if jisRune, mapped := cp932ToJIS[r]; mapped && jis {
						r = jisRune
					}
					out.WriteRune(r)
					i += 2
					continue
				}
			}
			// An invalid lead replaces itself alone; its trail byte is read next.
			out.WriteRune(utf8.RuneError)
			i++
		default:
			out.WriteRune(utf8.RuneError)
			i++
		}
	}
	return out.String()
}

func decodeJapanesePair(lead, trail byte) (rune, bool) {
	if lead >= 0xF0 && lead <= 0xF9 {
		// The user-defined rows of code page 932 map onto the private use area.
		n := int(trail) - 0x40
		if trail > 0x7E {
			n--
		}
		return rune(0xE000 + (int(lead)-0xF0)*188 + n), true
	}
	decoded, err := japanese.ShiftJIS.NewDecoder().Bytes([]byte{lead, trail})
	if err != nil {
		return 0, false
	}
	r, size := utf8.DecodeRune(decoded)
	if r == utf8.RuneError || size != len(decoded) {
		return 0, false
	}
	return r, true
}
