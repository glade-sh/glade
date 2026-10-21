package soql

import "testing"

func TestSOQLBindFamilyExpressionTokens(t *testing.T) {
	for _, query := range []string{
		"SELECT Id FROM Account WHERE Name LIKE :(p+'%')",
		"SELECT Id FROM Account WHERE Id IN :(new Map<Id,Account>(a)).keySet()",
		"SELECT Id FROM Account WHERE Id IN (:a[0].Id,:a[1].Id)",
		"SELECT Id FROM Account WHERE Id=: a [ 0 ] . Id",
		"SELECT Id FROM Account WHERE Name=:(b?'Acme1':'missing')",
		"SELECT Id FROM Account WHERE Name=:flag ? 'Acme1' : 'missing'",
		"SELECT Id FROM Account LIMIT : ((v))",
		"SELECT Id FROM Account WHERE Name='Approve:Thing'",
		"SELECT Id FROM Account WHERE CreatedDate=LAST_N_DAYS:1",
	} {
		if _, err := Parse(query); err != nil {
			t.Errorf("%s: %v", query, err)
		}
	}
}

func TestSOQLBindSafeNavigationExpressionTokens(t *testing.T) {
	cases := []struct {
		query string
		want  string
	}{
		{"SELECT Id FROM Account WHERE Name = :holder?.Name", "holder?.Name"},
		{"SELECT Id FROM Account WHERE Name = :holder?.b?.Name", "holder?.b?.Name"},
		{"SELECT Id FROM Account WHERE Name = :holder?.Name + 'x'", "holder?.Name + 'x'"},
		{"SELECT Id FROM Account WHERE Name = :names?.get(0)", "names?.get(0)"},
		{"SELECT Id FROM Account WHERE Name = :values?.get('k')", "values?.get('k')"},
		{"SELECT Id FROM Account WHERE Name = :holder ?. Name", "holder ?. Name"},
		{"SELECT Id FROM Account WHERE Name = :flag ? 'A' : 'B'", "flag ? 'A' : 'B'"},
		{"SELECT Id FROM Account WHERE Id IN :holder?.ids", "holder?.ids"},
		{"SELECT Id FROM Account LIMIT :holder?.n", "holder?.n"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			operands, err := BindExpressions(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(operands) != 1 || operands[0].Text != tc.want {
				t.Fatalf("bind operands = %#v, want %q", operands, tc.want)
			}
			if _, err := Parse(tc.query); err != nil {
				t.Fatalf("Parse(%q): %v", tc.query, err)
			}
		})
	}
	operand := "holder?.Name"
	end, ok := scanSOQLBindAdditiveOperand(operand, 0)
	if !ok || end != len(operand) {
		t.Fatalf("additive operand boundary = %d/%v, want %d/true", end, ok, len(operand))
	}
	spaced := "?. Name WHERE"
	if end := scanSOQLBindDottedTail(spaced, 0); end != len("?. Name") {
		t.Fatalf("spaced dotted tail boundary = %d, want %d", end, len("?. Name"))
	}
	operands, err := BindExpressions("SELECT Id FROM Account WHERE Name = 'literal?mark'")
	if err != nil || len(operands) != 0 {
		t.Fatalf("quoted question mark bind operands = %#v, err=%v; want none", operands, err)
	}
}
