package apextest

import "testing"

func TestSOQLBindLexicalOrgConformance(t *testing.T) {
	runSOQLBindNativeConformance(t, "soql_bind_lexical.json", "SOQLBindLexical", 87, 624)
}
