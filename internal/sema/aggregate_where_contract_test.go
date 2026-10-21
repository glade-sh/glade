package sema

import "testing"

// Check-only native deploys at APIs 65/66/67/68 reject COUNT in WHERE and
// accept both a scalar date function in WHERE and COUNT in HAVING.
func TestAggregateWhereContractAndValidNeighbors(t *testing.T) {
	for _, api := range []string{"65.0", "66.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, body string
			reject     bool
		}{
			{"where", "List<Account> rows=[SELECT Id FROM Account WHERE COUNT(Id)>1];", true},
			{"scalar", "List<Account> rows=[SELECT Id FROM Account WHERE CALENDAR_YEAR(CreatedDate)>2020];", false},
			{"having", "List<AggregateResult> rows=[SELECT Name,COUNT(Id) n FROM Account GROUP BY Name HAVING COUNT(Id)>1];", false},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"SemanticProbe.cls": "public class SemanticProbe { public static void observe() { " + tc.body + " } }",
				}, api)
				if result.HasErrors() != tc.reject || tc.reject && !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
					t.Fatalf("reject=%v, diagnostics=%#v", tc.reject, result.Diagnostics)
				}
			})
		}
	}
}
