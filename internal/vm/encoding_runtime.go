package vm

import "encoding/base64"

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
