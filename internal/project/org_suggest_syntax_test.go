package project

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupSuggestionSyntaxNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/org_setup_syntax"
	data, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Route, Class, Want string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 5 || row[3] != "OBSERVED" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, ok := observed[key]; ok {
			t.Fatalf("duplicate native row: %s", key)
		}
		observed[key] = row[4]
	}
	if len(observed) != len(cases)*4 {
		t.Fatalf("incomplete native route matrix: %d", len(observed))
	}
	for _, tc := range cases {
		data, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes", tc.Class+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		t.Run("sourceMatcher/"+tc.Route, func(t *testing.T) {
			for _, rule := range featureEvidenceRules {
				if matchesSetupFeature(rule, source, false) {
					t.Errorf("inert syntax generated %s advice", rule.feature)
				}
			}
		})
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"anonymous", "isTest"} {
				if got := observed[tc.Route+"/"+api+"/"+route]; got != tc.Want {
					t.Fatalf("native %s/%s/%s=%q, want %q", tc.Route, api, route, got, tc.Want)
				}
			}
			t.Run("SuggestOrgSetup/"+api+"/"+tc.Route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", tc.Class+".cls"), source)
				if got := SuggestOrgSetup(root); len(got) != 0 {
					t.Fatalf("inert syntax generated setup advice: %+v", got)
				}
			})
		}
	}
}

func TestSetupSuggestionCommentAndCallTokenRoutes(t *testing.T) {
	for _, tc := range []struct{ route, source, feature string }{
		{"lineComment", "// Site.getSiteId()\n", ""},
		{"blockComment", "/* Site.getSiteId() */", ""},
		{"quotedCall", "String label = 'Site.getSiteId()';", ""},
		{"quotedComment", "String label = '// Site.getSiteId()';", ""},
		{"jsComment", "// Site.getSiteId()\nconst label = 'plain';", ""},
		{"callIdentifier", "Object value = Site.getSiteId();", "Sites"},
		{"callWhitespace", "Object value = Site /* spacer */ . getSiteId ();", "Sites"},
		{"unexecutedCommunityQuery", "String label = 'SELECT IsPortalEnabled FROM User';", ""},
		{"unexecutedTerritoryQuery", "String label = 'SELECT Id FROM Territory2';", ""},
	} {
		t.Run(tc.route, func(t *testing.T) {
			for _, rule := range featureEvidenceRules {
				if got, want := matchesSetupFeature(rule, tc.source, false), rule.feature == tc.feature; got != want {
					t.Errorf("%s evidence=%v, want %v", rule.feature, got, want)
				}
			}
		})
	}
}

func TestSetupSuggestionAggregateQueryFieldRoutes(t *testing.T) {
	for _, tc := range []struct{ route, query, feature string }{
		{"personAlias", "SELECT COUNT(Id) PersonEmail FROM Account", ""},
		{"currencyAlias", "SELECT COUNT(Id) CurrencyIsoCode FROM Opportunity", ""},
		{"nativeUserCurrencyField", "SELECT DefaultCurrencyIsoCode FROM User", "MultiCurrency"},
		{"actualPersonAggregateField", "SELECT COUNT(PersonEmail) total FROM Account", "PersonAccounts"},
		{"actualCurrencyGroupField", "SELECT COUNT(Id) total, CurrencyIsoCode FROM Opportunity GROUP BY CurrencyIsoCode", "MultiCurrency"},
		{"actualPersonFilter", "SELECT Id FROM Account WHERE PersonEmail != null", "PersonAccounts"},
		{"actualPersonOrder", "SELECT Id FROM Account ORDER BY PersonEmail", "PersonAccounts"},
		{"bindName", "SELECT Id FROM Account WHERE Name = :PersonEmail", ""},
	} {
		for _, method := range []string{"static", "query", "queryWithBinds", "getQueryLocator", "getQueryLocatorWithBinds", "countQuery", "countQueryWithBinds"} {
			t.Run(method+"/"+tc.route, func(t *testing.T) {
				source := "Object rows = [" + tc.query + "];"
				if method != "static" {
					source = "Object rows = Database." + method + "('" + tc.query + "');"
				}
				for _, rule := range featureEvidenceRules {
					if rule.feature != "PersonAccounts" && rule.feature != "MultiCurrency" {
						continue
					}
					if got, want := matchesSetupFeature(rule, source, false), rule.feature == tc.feature; got != want {
						t.Errorf("%s evidence=%v, want %v", rule.feature, got, want)
					}
				}
			})
		}
	}
}

func TestSetupSuggestionExecutedCommunityTerritoryRoutes(t *testing.T) {
	for _, tc := range []struct{ feature, query string }{
		{"Communities", "SELECT IsPortalEnabled FROM User"},
		{"Territory2", "SELECT Id FROM Territory2"},
	} {
		for _, method := range []string{"static", "query", "queryWithBinds", "getQueryLocator", "getQueryLocatorWithBinds", "countQuery", "countQueryWithBinds"} {
			source := "Object rows = [" + tc.query + "];"
			if method != "static" {
				source = "Object rows = Database." + method + "('" + tc.query + "');"
			}
			for _, context := range []string{"code", "literal", "comment"} {
				t.Run(tc.feature+"/"+method+"/"+context, func(t *testing.T) {
					input := source
					if context == "literal" {
						input = `String label = "` + source + `";`
					}
					if context == "comment" {
						input = "/* " + source + " */"
					}
					for _, rule := range featureEvidenceRules {
						if rule.feature == tc.feature && matchesSetupFeature(rule, input, false) != (context == "code") {
							t.Fatalf("%s source context evidence mismatch", context)
						}
					}
				})
			}
		}
	}
}
