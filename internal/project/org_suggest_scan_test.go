package project

import (
	"path/filepath"
	"testing"
)

func TestSetupQueryScreenPreservesParsedEvidence(t *testing.T) {
	t.Run("compound custom keys", testCompoundCustomWitnesses)
	t.Run("Unicode before lowercase", func(t *testing.T) {
		if !queryCouldUseSetupFeature("SELECT Id FROM Account WHERE Name = 'K'", "MultiCurrency", nil) {
			t.Fatal("original non-ASCII input bypassed the parser")
		}
	})
	custom := map[string]bool{"color__pc": true, "arbitrary": true}
	queries := []string{
		"SELECT Id FROM Account", "SELECT IsPersonAccount, PersonContactId FROM Account",
		"SELECT CurrencyIsoCode FROM Opportunity", "SELECT Id FROM DatedConversionRate",
		"SELECT Id FROM CurrencyType", "SELECT convertCurrency(Amount) FROM Opportunity",
		"SELECT a.Color__pc FROM Account a", "SELECT arbitrary FROM Account", "SELECT Name FROM Contact",
		"SELECT MAX(CurrencyIsoCode) x FROM Account GROUP BY Name ORDER BY x",
		"SELECT Id,(SELECT CurrencyIsoCode FROM Contacts) FROM Account",
		"SELECT CurrencyIsoCode LIKE 'a' IsPersonAccount nonsense", "SELECT Id FROM Account WHERE Name='CurrencyType'",
		"SELECT Id FROM Account WHERE Name='Grüße'", "[SELECT currencyisocode FROM Opportunity]",
		"SELECT IsPortalEnabled FROM User", "SELECT Amount FROM Opportunity", "",
	}
	for _, q := range queries {
		for _, feature := range []string{"PersonAccounts", "MultiCurrency", "FutureFeature"} {
			got := setupQueryUsesPlatformFeature(q, feature, custom)
			want := parsedSetupQueryUsesFeature(q, feature, custom)
			if got != want {
				t.Fatalf("different feature result for %s, query %q", feature, q)
			}
		}
	}
}

func testCompoundCustomWitnesses(t *testing.T) {
	var p Project
	for i, name := range []string{"Region.Loyalty__c", "Price$Tier__c"} {
		file := filepath.Join(t.TempDir(), "objects", "Contact", "fields", []string{"Region", "Tier"}[i]+".field-meta.xml")
		writeFile(t, file, "<CustomField><fullName>"+name+"</fullName></CustomField>")
		p.FieldFiles = append(p.FieldFiles, file)
	}
	custom := setupCustomPersonAccountFields(p)
	if !custom["region.loyalty__pc"] || !custom["price$tier__pc"] {
		t.Fatalf("expected exact projected names, got %v", custom)
	}
	rule := featureEvidence{feature: "PersonAccounts", customPersonFields: custom}
	for _, q := range []string{"SELECT a.Region.Loyalty__pc FROM Account a", "SELECT Price$Tier__pc FROM Account"} {
		if !parsedSetupQueryUsesFeature(q, rule.feature, custom) {
			t.Fatalf("baseline parser did not accept %q", q)
		}
		if !queryCouldUseSetupFeature(q, rule.feature, custom) || !setupQueryUsesPlatformFeature(q, rule.feature, custom) {
			t.Fatalf("positive field screened out: %q", q)
		}
		if !matchesSetupFeature(rule, "class Probe { void run(){ Database.query('"+q+"'); } }", false) {
			t.Fatalf("dynamic query lost field: %q", q)
		}
	}
	// Unsuffixed arbitrary keys use the same helper contract as ordinary tokens.
	if q := "SELECT a.Scope.Hint FROM Account a"; !parsedSetupQueryUsesFeature(q, rule.feature, map[string]bool{"scope.hint": true}) ||
		!setupQueryUsesPlatformFeature(q, rule.feature, map[string]bool{"scope.hint": true}) {
		t.Fatal("unrelated suffix required for dotted custom field")
	}
	joined := "class Probe { void run(){ Database.query('SELECT a.Region.' + 'Loyalty__pc FROM Account a'); } }"
	if !matchesSetupFeature(rule, joined, false) {
		t.Fatal("literal concatenation lost the complete field")
	}
	for _, q := range []string{"SELECT x.Region.Loyalty__pc FROM Account a", "SELECT Id FROM Account WHERE Name='Price$Tier__pc'", "SELECT COUNT(Id) Price$Tier__pc FROM Account", "SELECT a.Region.Loyalty__pc FROM Contact a"} {
		if parsedSetupQueryUsesFeature(q, rule.feature, custom) || setupQueryUsesPlatformFeature(q, rule.feature, custom) {
			t.Fatalf("unexpected evidence %q", q)
		}
	}
	if matchesSetupFeature(rule, "class Probe { void run(){ /* Database.query('SELECT Price$Tier__pc FROM Account'); */ } }", false) {
		t.Fatal("comment supplied evidence")
	}
}
