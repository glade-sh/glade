package soql

import "testing"

func TestSOQLBindCapturedSyntaxDiagnostics(t *testing.T) {
	for _, test := range []struct{ query, token string }{
		{"SELECT Id FROM Account WHERE Name=:()", ")"},
		{"SELECT Id FROM Account WHERE Id IN (:a[0].Id,)", ")"},
		{"SELECT Id FROM Account ORDER BY :v", ":"},
		{"SELECT Id FROM Account ORDER BY :(v)", ":"},
		{"SELECT Id FROM Account ORDER BY :v[0]", ":"},
	} {
		_, err := Parse(test.query)
		if got := BindCompileDiagnosticMessage(err); got != "Unexpected token '"+test.token+"'." {
			t.Fatalf("%s: <%s> (%v)", test.query, got, err)
		}
	}
	if _, err := Parse("SELECT Id FROM Account ORDER BY Name"); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("SELECT Id FROM Account WHERE Id IN (:a[0].Id,:a[1].Id)"); err != nil {
		t.Fatal(err)
	}
}
