package project

import "testing"

func TestSetupFeatureMatcherDatabaseQueryEntryPointRoutes(t *testing.T) {
	for _, method := range []string{"query", "queryWithBinds", "getQueryLocator", "getQueryLocatorWithBinds", "countQuery", "countQueryWithBinds"} {
		for _, tc := range []struct{ feature, query string }{
			{"PersonAccounts", "SELECT IsPersonAccount FROM Account"},
			{"MultiCurrency", "SELECT CurrencyIsoCode FROM Opportunity"},
		} {
			t.Run("sourceMatcher/Database."+method+"/"+tc.feature, func(t *testing.T) {
				for _, rule := range featureEvidenceRules {
					if rule.feature != tc.feature {
						continue
					}
					source := "Object rows = Database." + method + "('" + tc.query + "');"
					if !matchesSetupFeature(rule, source, false) {
						t.Fatal("query entry point lost platform field evidence")
					}
					label := `String label = "` + source + `";`
					if matchesSetupFeature(rule, label, false) {
						t.Fatal("quoted query call generated advice")
					}
				}
			})
		}
	}
}
