package sema

import "testing"

// Named Salesforce deployments accept the AccessLevel overload and reject a
// Boolean argument at source APIs 63, 65, 67 and 68.
func TestSearchQueryObservedAccessLevelOverload(t *testing.T) {
	for _, api := range []string{"63.0", "65.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, test := range []struct {
				name, body string
				wantError  bool
			}{
				{"typed", `public static List<List<SObject>> observe(String queryText, System.AccessLevel level) { return Search.query(queryText, level); }`, false},
				{"literal", `public static List<List<SObject>> observe(String queryText) { return System.Search.query(queryText, AccessLevel.USER_MODE); }`, false},
				{"boolean", `public static List<List<SObject>> observe(String queryText) { return Search.query(queryText, true); }`, true},
			} {
				t.Run(test.name, func(t *testing.T) {
					result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"SearchModeProbe.cls": "public class SearchModeProbe {" + test.body + "}"}, api)
					if result.HasErrors() != test.wantError {
						t.Fatalf("errors = %v, want %v: %#v", result.HasErrors(), test.wantError, result.Diagnostics)
					}
				})
			}
		})
	}
}
