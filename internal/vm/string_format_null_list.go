package vm

import (
	"fmt"
	"strconv"
	"strings"
)

// A null argument list retains literal quoting and leaves argument slots unfilled.
// Braced slots are validated even when their argument list is absent.
func stringFormatNullList(pattern, arguments Value) (Value, bool, error) {
	if pattern.Kind != ValueString || arguments.Kind != ValueNull {
		return Null, false, nil
	}
	var out strings.Builder
	quoted := false
	for i := 0; i < len(pattern.Text); i++ {
		ch := pattern.Text[i]
		if ch == '\'' {
			if end := strings.IndexByte(pattern.Text[i+1:], '\''); end >= 0 {
				literal := pattern.Text[i+1 : i+1+end]
				if strings.ContainsAny(literal, "{}") {
					out.WriteString(literal)
					i += end + 1
					continue
				}
			}
			quoted = !quoted
			out.WriteByte(ch)
			continue
		}
		if ch != '{' || quoted {
			out.WriteByte(ch)
			continue
		}
		end, depth := i+1, 1
		for ; end < len(pattern.Text); end++ {
			if pattern.Text[end] == '{' {
				depth++
			}
			if pattern.Text[end] == '}' {
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if depth != 0 {
			return Null, true, newExceptionError("StringException", fmt.Sprintf("Unmatched '{' braces in message %q", pattern.Text))
		}
		token := pattern.Text[i+1 : end]
		indexText := strings.TrimSpace(strings.SplitN(token, ",", 2)[0])
		index, err := strconv.Atoi(indexText)
		if err != nil || index < 0 {
			return Null, true, newExceptionError("StringException", "This method is not available in MessageFormat objects that use alphanumeric argument names.")
		}
		if _, _, typed, err := formatStringTypedTokenParts(token); typed && err != nil {
			return Null, true, err
		}
		out.WriteByte('{')
		out.WriteString(strconv.Itoa(index))
		out.WriteByte('}')
		i = end
	}
	return String(out.String()), true, nil
}
