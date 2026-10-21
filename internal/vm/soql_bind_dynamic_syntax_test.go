package vm

import "testing"

func TestDynamicSOQLBindSyntaxCapturedBoundaries(t *testing.T) {
	machine := New(nil)
	for _, test := range []struct {
		name, query string
		reject      bool
	}{
		{"mapValues", "SELECT Id FROM Account WHERE Id IN :accountsById.values()", false},
		{"stringUpperCase", "SELECT Id FROM Account WHERE Name=:v.toUpperCase()", true},
		{"parenthesized", "SELECT Id FROM Account WHERE Name=:(v)", true},
		{"indexed", "SELECT Id FROM Account WHERE Name=:values[0]", true},
		{"orderBy", "SELECT Id FROM Account ORDER BY :fieldName", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, countQuery := range []bool{false, true} {
				err := machine.validateDynamicSOQLBindSyntax(test.query, countQuery)
				if (err != nil) != test.reject {
					t.Fatalf("countQuery=%v: reject=%v, error=%v", countQuery, test.reject, err)
				}
			}
		})
	}
}
