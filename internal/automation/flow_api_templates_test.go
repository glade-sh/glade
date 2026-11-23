package automation

import "testing"

// The API endpoint formula rows cover the direct reference and the origin
// expression. Other expression shapes and endpoint versions stay unselected.
func TestFlowAPIFormulaSelectionBoundaries(t *testing.T) {
	cases := []struct {
		expression string
		want       bool
	}{
		{`$Api.Partner_Server_URL_560`, true},
		{`$Api.Enterprise_Server_URL_560`, true},
		{`LEFT($Api.Partner_Server_URL_560, FIND("/services", $Api.Partner_Server_URL_560) - 1)`, true},
		{`SUBSTITUTE(LEFT($Api.Partner_Server_URL_560,FIND("/services",$Api.Partner_Server_URL_560)-1),"x","y") + "/path"`, true},
		{`$Api.Partner_Server_URL_550`, false},
		{`UPPER($Api.Partner_Server_URL_560)`, false},
		{`LEFT($Api.Partner_Server_URL_560, 3)`, false},
	}
	for _, tc := range cases {
		if got := capturedTemplateAPIFormula(tc.expression); got != tc.want {
			t.Errorf("%q selected=%v, want %v", tc.expression, got, tc.want)
		}
	}
}
