package vm

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func decodeApexBase64(text string) ([]byte, error) {
	// Apex ignores padding and observed transport whitespace, including padding
	// before or between symbols. Validate every symbol before dropping an
	// incomplete final sextet; even a one-character invalid input must throw.
	symbols := make([]byte, 0, len(text))
	for _, char := range text {
		switch {
		case char == '=' || char == ' ' || char == '\r' || char == '\n' || char == '\t' || char == '\f' || char == '\v':
			continue
		case char >= 'A' && char <= 'Z', char >= 'a' && char <= 'z', char >= '0' && char <= '9', char == '+', char == '/':
			symbols = append(symbols, byte(char))
		default:
			return nil, newExceptionError("System.StringException", "Unrecognized base64 character: "+string(char))
		}
	}
	if len(symbols)%4 == 1 {
		symbols = symbols[:len(symbols)-1]
	}
	return base64.RawStdEncoding.DecodeString(string(symbols))
}

func decodeApexHex(text string) ([]byte, error) {
	// Apex counts and indexes UTF-16 characters, and accepts fullwidth digits.
	units := utf16.Encode([]rune(text))
	if len(units)%2 != 0 {
		return nil, newExceptionError("System.InvalidParameterValueException", fmt.Sprintf("input string must be an even number of characters long, but was %d", len(units)))
	}
	decoded := make([]byte, len(units)/2)
	for i, unit := range units {
		char := rune(unit)
		if char >= '\uff10' && char <= '\uff19' || char >= '\uff21' && char <= '\uff26' || char >= '\uff41' && char <= '\uff46' {
			char -= 0xfee0
		}
		var digit byte
		var ok bool
		if char <= 0x7f {
			digit, ok = fromHex(byte(char))
		}
		if !ok {
			return nil, newExceptionError("System.InvalidParameterValueException", fmt.Sprintf("Illegal hexadecimal character %c at index %d", unit, i))
		}
		if i%2 == 0 {
			decoded[i/2] = digit << 4
		} else {
			decoded[i/2] |= digit
		}
	}
	return decoded, nil
}

func normalizeURLCharset(charset string) string {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(charset), "_", "-"))
	switch normalized {
	case "utf8":
		return "utf-8"
	case "usascii", "ascii":
		return "us-ascii"
	case "iso8859-1", "iso-88591", "iso88591", "latin1", "latin-1":
		return "iso-8859-1"
	case "utf16":
		return "utf-16"
	case "utf16be":
		return "utf-16be"
	case "utf16le":
		return "utf-16le"
	default:
		return normalized
	}
}

func urlCharsetEncoding(charset string) (encoding.Encoding, error) {
	switch normalizeURLCharset(charset) {
	case "utf-8":
		return textunicode.UTF8, nil
	case "us-ascii":
		return nil, nil // ASCII uses one replacement per unmappable character.
	case "iso-8859-1":
		return charmap.ISO8859_1, nil
	case "windows-1252":
		return charmap.Windows1252, nil
	case "utf-16":
		return textunicode.UTF16(textunicode.BigEndian, textunicode.UseBOM), nil
	case "utf-16be":
		return textunicode.UTF16(textunicode.BigEndian, textunicode.IgnoreBOM), nil
	case "utf-16le":
		return textunicode.UTF16(textunicode.LittleEndian, textunicode.IgnoreBOM), nil
	case "shift-jis":
		return japanese.ShiftJIS, nil
	case "gb18030":
		return simplifiedchinese.GB18030, nil
	default:
		return nil, fmt.Errorf("Encoding %s is not supported", charset)
	}
}

func urlEncodeWithCharset(_ string, text, charset string) (string, error) {
	codec, err := urlCharsetEncoding(charset)
	if err != nil {
		return "", err
	}
	charset = normalizeURLCharset(charset)
	var out strings.Builder
	// Encode each contiguous unsafe character span, then escape every encoded
	// byte. Safe ASCII and spaces are handled before conversion, even in UTF-16.
	flush := func(start, end int) error {
		if start == end {
			return nil
		}
		encoded, err := encodeURLCharsetBytes(text[start:end], charset, codec)
		if err != nil {
			return err
		}
		for _, b := range encoded {
			writeURLPercentByte(&out, b)
		}
		return nil
	}
	start := 0
	for i, r := range text {
		if !isURLEncodedSafeASCII(r) && r != ' ' {
			continue
		}
		if err := flush(start, i); err != nil {
			return "", err
		}
		if r == ' ' {
			out.WriteByte('+')
		} else {
			out.WriteByte(byte(r))
		}
		start = i + 1 // Only single-byte ASCII enters this branch.
	}
	if err := flush(start, len(text)); err != nil {
		return "", err
	}
	return out.String(), nil
}

func encodeURLCharsetBytes(text, charset string, codec encoding.Encoding) ([]byte, error) {
	switch charset {
	case "us-ascii", "iso-8859-1", "windows-1252":
		out := make([]byte, 0, len(text))
		for _, r := range text {
			b, ok := byte('?'), false
			switch charset {
			case "us-ascii":
				b, ok = byte(r), r <= 0x7f
			case "iso-8859-1":
				b, ok = charmap.ISO8859_1.EncodeRune(r)
			case "windows-1252":
				b, ok = charmap.Windows1252.EncodeRune(r)
			}
			if !ok {
				b = '?'
			}
			out = append(out, b)
		}
		return out, nil
	default:
		// Apex uses '?' for unmappable source characters. Keep successfully
		// encoded bytes, including literal ASCII SUB, unchanged.
		encoder := codec.NewEncoder()
		var out []byte
		for remaining := []byte(text); len(remaining) > 0; {
			var consumed int
			var err error
			out, consumed, err = transform.Append(encoder, out, remaining)
			if err == nil {
				return out, nil
			}
			if _, unmappable := err.(interface{ Replacement() byte }); !unmappable || consumed >= len(remaining) {
				return nil, err
			}
			_, size := utf8.DecodeRune(remaining[consumed:])
			out = append(out, '?')
			remaining = remaining[consumed+size:]
		}
		return out, nil
	}
}

func isURLEncodedSafeASCII(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == '*'
}

func writeURLPercentByte(out *strings.Builder, b byte) {
	const digits = "0123456789ABCDEF"
	out.WriteByte('%')
	out.WriteByte(digits[b>>4])
	out.WriteByte(digits[b&0x0f])
}

func urlDecodeWithCharset(_ string, text, charset string) (string, error) {
	codec, err := urlCharsetEncoding(charset)
	if err != nil {
		return "", err
	}
	charset = normalizeURLCharset(charset)
	var out strings.Builder
	for i := 0; i < len(text); {
		switch text[i] {
		case '+':
			out.WriteByte(' ')
			i++
		case '%':
			var escaped []byte
			for i < len(text) && text[i] == '%' {
				if i+2 >= len(text) {
					return "", fmt.Errorf("URLDecoder: Incomplete trailing escape (%%) pattern")
				}
				pair := text[i+1 : i+3]
				hi, hiOK := fromHex(pair[0])
				lo, loOK := fromHex(pair[1])
				if !hiOK || !loOK {
					index := 0
					if hiOK {
						index = 1
					}
					return "", fmt.Errorf("URLDecoder: Illegal hex characters in escape (%%) pattern - Error at index %d in: %q", index, pair)
				}
				escaped = append(escaped, hi<<4|lo)
				i += 3
			}
			decoded, err := decodeURLCharsetBytes(escaped, charset, codec)
			if err != nil {
				return "", err
			}
			out.WriteString(decoded)
		default:
			out.WriteByte(text[i])
			i++
		}
	}
	return out.String(), nil
}

func decodeURLCharsetBytes(data []byte, charset string, codec encoding.Encoding) (string, error) {
	switch charset {
	case "utf-8":
		return decodeURLUTF8(data), nil
	case "us-ascii":
		var out strings.Builder
		for _, b := range data {
			if b <= 0x7f {
				out.WriteByte(b)
			} else {
				out.WriteRune(utf8.RuneError)
			}
		}
		return out.String(), nil
	default:
		decoded, err := codec.NewDecoder().Bytes(data)
		return string(decoded), err
	}
}

func decodeURLUTF8(data []byte) string {
	var out strings.Builder
	for i := 0; i < len(data); {
		r, n := utf8.DecodeRune(data[i:])
		if r != utf8.RuneError || n != 1 {
			out.WriteRune(r)
			i += n
			continue
		}
		// Replace a malformed sequence once, including a complete encoded
		// surrogate (R201). Invalid leads such as C0/AF each replace once (R200).
		lead := data[i]
		width := 1
		switch {
		case lead >= 0xc2 && lead <= 0xdf:
			width = 2
		case lead >= 0xe0 && lead <= 0xef:
			width = 3
		case lead >= 0xf0 && lead <= 0xf4:
			width = 4
		}
		n = 1
		for n < width && i+n < len(data) {
			b := data[i+n]
			if b < 0x80 || b > 0xbf {
				break
			}
			if n == 1 && (lead == 0xe0 && b < 0xa0 || lead == 0xf0 && b < 0x90 || lead == 0xf4 && b > 0x8f) {
				break
			}
			n++
		}
		out.WriteRune(utf8.RuneError)
		i += n
	}
	return out.String()
}

func fromHex(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}
