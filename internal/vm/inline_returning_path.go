package vm

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
)

// Preserve the measured inline Contact(Id, parent.field) token adjacency.
// Spaced dots, dynamic queries and other RETURNING shapes retain their paths.
func compactCapturedInlineReturningParts(parts []string, tokens []token, api string) []string {
	if !apexversion.AtLeast(api, 62) || apexversion.AtLeast(api, 69) {
		return parts
	}
	if len(parts) == 20 && len(tokens) == 20 {
		return compactBoundParentReturningParts(parts, tokens)
	}
	if len(parts) != 14 || len(tokens) != 14 {
		return parts
	}
	for index, word := range map[int]string{0: "FIND", 2: "IN", 3: "ALL", 4: "FIELDS", 5: "RETURNING", 6: "Contact", 8: "Id"} {
		if tokens[index].kind != tokenIdent || !strings.EqualFold(tokens[index].text, word) {
			return parts
		}
	}
	if tokens[1].kind != tokenString || tokens[7].text != "(" || tokens[9].text != "," || tokens[13].text != ")" ||
		tokens[10].kind != tokenIdent || tokens[11].text != "." || tokens[12].kind != tokenIdent ||
		tokens[10].pos+len(tokens[10].text) != tokens[11].pos || tokens[11].pos+1 != tokens[12].pos {
		return parts
	}
	out := append([]string(nil), parts[:10]...)
	out = append(out, parts[10]+"."+parts[12], parts[13])
	return out
}

// PENDING-ROWS: preserve adjacent parent.field tokens in only the bound,
// two-object ALL/NAME search shapes recorded in the pending capture packet.
// Other fields, query shapes and source-spaced dots retain their current paths.
func compactBoundParentReturningParts(parts []string, tokens []token) []string {
	for index, word := range map[int]string{0: "FIND", 3: "IN", 5: "FIELDS", 6: "RETURNING", 7: "Account", 9: "Name", 12: "Contact", 14: "LastName", 16: "Account", 18: "Name"} {
		if tokens[index].kind != tokenIdent || !strings.EqualFold(tokens[index].text, word) {
			return parts
		}
	}
	if tokens[1].text != ":" || tokens[2].kind != tokenIdent ||
		(!strings.EqualFold(tokens[4].text, "ALL") && !strings.EqualFold(tokens[4].text, "NAME")) {
		return parts
	}
	for index, symbol := range map[int]string{8: "(", 10: ")", 11: ",", 13: "(", 15: ",", 17: ".", 19: ")"} {
		if tokens[index].text != symbol {
			return parts
		}
	}
	if tokens[16].pos+len(tokens[16].text) != tokens[17].pos || tokens[17].pos+1 != tokens[18].pos {
		return parts
	}
	out := append([]string(nil), parts[:16]...)
	return append(out, parts[16]+"."+parts[18], parts[19])
}
