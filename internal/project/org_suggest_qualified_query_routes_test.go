package project

import "testing"

// The custom field's root qualifier is captured in
// conformance/org_setup_custom_person_query/org.tsv:10-11,44-45. The query
// parser uses the same storage-alias node for custom and catalogued fields.
func TestSetupQualifiedQueryEntryPointRoutes(t *testing.T) {
	for _, tc := range []struct{ route, feature, object, field string }{
		{"customPerson", "PersonAccounts", "Account", "Loyalty__pc"},
		{"catalogPerson", "PersonAccounts", "Account", "IsPersonAccount"},
		{"catalogCurrency", "MultiCurrency", "Opportunity", "CurrencyIsoCode"},
	} {
		for _, method := range []string{"static", "query", "queryWithBinds", "getQueryLocator", "getQueryLocatorWithBinds", "countQuery", "countQueryWithBinds"} {
			t.Run(tc.route+"/"+method, func(t *testing.T) {
				query := "SELECT r." + tc.field + " FROM " + tc.object + " r WHERE Id = null"
				if method == "countQuery" || method == "countQueryWithBinds" {
					query = "SELECT COUNT() FROM " + tc.object + " r WHERE r." + tc.field + " != null AND Id = null"
				}
				source := "Object rows = [" + query + "];"
				if method != "static" {
					args := "'" + query + "'"
					if method == "queryWithBinds" || method == "getQueryLocatorWithBinds" || method == "countQueryWithBinds" {
						args += ", new Map<String, Object>(), AccessLevel.SYSTEM_MODE"
					}
					source = "Object rows = Database." + method + "(" + args + ");"
				}
				for _, rule := range featureEvidenceRules {
					rule.customPersonFields = map[string]bool{"loyalty__pc": true}
					if got, want := matchesSetupFeature(rule, source, false), rule.feature == tc.feature; got != want {
						t.Errorf("qualified %s query: %s evidence=%v, want %v", method, rule.feature, got, want)
					}
				}
			})
		}
	}
}

func TestSetupQualifiedAggregateAliasControls(t *testing.T) {
	for _, query := range []string{
		"SELECT COUNT(Id) Loyalty__pc FROM Account a",
		"SELECT COUNT(Id) PersonEmail FROM Account a",
		"SELECT COUNT(Id) CurrencyIsoCode FROM Opportunity o",
	} {
		for _, feature := range []string{"PersonAccounts", "MultiCurrency"} {
			if setupQueryUsesPlatformFeature(query, feature, map[string]bool{"loyalty__pc": true}) {
				t.Errorf("aggregate alias generated %s advice: %s", feature, query)
			}
		}
	}
}
