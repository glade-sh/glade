package vm

import (
	"encoding/json"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// json_lone_surrogate_c4 (API 62.0 and 67.0): JSON.deserialize,
// deserializeUntyped and JSONParser keep a lone surrogate, written either as a
// \uD800-style escape or as the raw unit (the three-byte form Glade stores),
// and join a high and a low unit into one code point whatever their spelling.
// encoding/json replaces both with U+FFFD, so the input carries each lone unit
// through the decoder as a code point of the last 2048 of plane 16 and the
// decoded strings map it back. Text that already holds such a code point keeps
// the encoding/json result.
const jsonLoneSurrogateCarrier = 0x10F800

type apexJSONDecoder struct {
	*json.Decoder
	carried bool
}

// newApexJSONDecoder returns a UseNumber decoder over text and the source its
// input offsets refer to.
func newApexJSONDecoder(text string) (*apexJSONDecoder, string) {
	prepared, carried := carryJSONLoneSurrogates(text)
	decoder := json.NewDecoder(strings.NewReader(prepared))
	decoder.UseNumber()
	return &apexJSONDecoder{Decoder: decoder, carried: carried}, prepared
}

func (d *apexJSONDecoder) Token() (json.Token, error) {
	token, err := d.Decoder.Token()
	if text, ok := token.(string); ok && d.carried {
		return restoreJSONLoneSurrogates(text), err
	}
	return token, err
}

func carryJSONLoneSurrogates(text string) (string, bool) {
	if !strings.Contains(text, `\u`) && !mayHoldLoneSurrogate(text) {
		return text, false
	}
	var b strings.Builder
	changed := false
	for i := 0; i < len(text); {
		switch {
		case isLoneSurrogateAt(text, i):
			unit, _ := decodeApexRune(text[i:])
			b.WriteRune(jsonLoneSurrogateCarrier + unit - 0xD800)
			i += 3
			changed = true
		case text[i] == '\\' && i+1 < len(text) && text[i+1] == 'u':
			unit, ok := jsonEscapeUnit(text, i)
			if !ok || unit < 0xD800 || unit > 0xDFFF {
				b.WriteString(text[i:min(i+6, len(text))])
				i = min(i+6, len(text))
				continue
			}
			if unit <= 0xDBFF {
				if low, ok := jsonEscapeUnit(text, i+6); ok && low >= 0xDC00 && low <= 0xDFFF {
					b.WriteString(text[i : i+12])
					i += 12
					continue
				}
			}
			b.WriteRune(jsonLoneSurrogateCarrier + rune(unit) - 0xD800)
			i += 6
			changed = true
		case text[i] == '\\' && i+1 < len(text):
			b.WriteString(text[i : i+2])
			i += 2
		default:
			r, size := utf8.DecodeRuneInString(text[i:])
			if r >= jsonLoneSurrogateCarrier {
				return text, false
			}
			b.WriteString(text[i : i+size])
			i += size
		}
	}
	if !changed {
		return text, false
	}
	return b.String(), true
}

func jsonEscapeUnit(text string, i int) (uint16, bool) {
	if i+6 > len(text) || text[i] != '\\' || text[i+1] != 'u' {
		return 0, false
	}
	var unit uint16
	for _, c := range []byte(text[i+2 : i+6]) {
		switch {
		case c >= '0' && c <= '9':
			unit = unit<<4 | uint16(c-'0')
		case c >= 'a' && c <= 'f':
			unit = unit<<4 | uint16(c-'a'+10)
		case c >= 'A' && c <= 'F':
			unit = unit<<4 | uint16(c-'A'+10)
		default:
			return 0, false
		}
	}
	return unit, true
}

func restoreJSONLoneSurrogates(text string) string {
	if !containsJSONCarrier(text) {
		return text
	}
	units := make([]uint16, 0, len(text))
	for _, r := range text {
		if r >= jsonLoneSurrogateCarrier {
			units = append(units, uint16(0xD800+r-jsonLoneSurrogateCarrier))
			continue
		}
		units = utf16.AppendRune(units, r)
	}
	return apexStringFromUTF16Units(units)
}

func containsJSONCarrier(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] == 0xF4 {
			if r, _ := utf8.DecodeRuneInString(text[i:]); r >= jsonLoneSurrogateCarrier {
				return true
			}
		}
	}
	return false
}
