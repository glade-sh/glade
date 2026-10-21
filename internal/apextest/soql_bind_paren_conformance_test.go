package apextest

import "testing"

func TestSOQLBindParenOrgConformance(t *testing.T) {
	runSOQLBindNativeConformance(t, "soql_bind_paren.json", "SOQLBindParen", 161, 1288)
}
