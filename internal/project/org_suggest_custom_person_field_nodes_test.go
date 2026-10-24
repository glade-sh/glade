package project

import "testing"

// org_setup_custom_person_query/org.tsv captures the Account field, root
// qualification, aggregate input, WHERE/ORDER BY, and aggregate-alias controls.
func TestCustomPersonQueryASTFieldNodeRoutes(t *testing.T) {
	fields := map[string]bool{"loyalty__pc": true}
	for _, tc := range []struct {
		route, query string
		want         bool
	}{
		{"selectField", "SELECT Loyalty__pc FROM Account", true},
		{"whereField", "SELECT Id FROM Account WHERE Loyalty__pc = null", true},
		{"orderField", "SELECT Id FROM Account ORDER BY Loyalty__pc", true},
		{"aggregateInput", "SELECT COUNT(Loyalty__pc) total FROM Account", true},
		{"qualifiedField", "SELECT a.Loyalty__pc FROM Account a", true},
		{"aggregateAlias", "SELECT COUNT(Id) Loyalty__pc FROM Account", false},
		{"otherObject", "SELECT Loyalty__pc FROM Contact", false},
		{"missingMetadata", "SELECT Missing__pc FROM Account", false},
	} {
		t.Run(tc.route, func(t *testing.T) {
			if got := setupQueryUsesPlatformFeature(tc.query, "PersonAccounts", fields); got != tc.want {
				t.Fatalf("field node evidence=%v, want %v", got, tc.want)
			}
			if setupQueryUsesPlatformFeature(tc.query, "PersonAccounts", nil) {
				t.Fatal("custom query field without metadata generated advice")
			}
			if setupQueryUsesPlatformFeature(tc.query, "MultiCurrency", fields) {
				t.Fatal("custom Person Account field generated currency advice")
			}
		})
	}
}
