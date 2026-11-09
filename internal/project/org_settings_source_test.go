package project

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func writeSourceSetting(t *testing.T, root, name, body string) string {
	t.Helper()
	path := filepath.Join(root, "force-app/main/default/settings", name+".settings-meta.xml")
	writeFile(t, path, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<"+name+`Settings xmlns="http://soap.sforce.com/2006/04/metadata">`+body+"</"+name+"Settings>\n")
	return path
}

// Each deployed settings file reaches the trigger table that scratch-definition
// settings use. The key and feature pairs follow
// conformance/org_settings_source/org.tsv.
func TestOrgShapeFeaturesReadsDeployedSettings(t *testing.T) {
	cases := []struct{ name, body, feature string }{
		{"Quote", "<enableQuote>true</enableQuote>", "Quotes"},
		{"Account", "<enableAccountTeams>true</enableAccountTeams>", "AccountTeams"},
		{"Opportunity", "<enableOpportunityTeam>true</enableOpportunityTeam>", "OpportunityTeam"},
		{"Territory2", "<enableTerritoryManagement2>true</enableTerritoryManagement2>", "Territory2"},
		{"Forecasting", "<enableForecasts>true</enableForecasts>", "Forecasts"},
		{"Currency", "<enableMultiCurrency>true</enableMultiCurrency>", "MultiCurrency"},
		{"Communities", "<enableNetworksEnabled>true</enableNetworksEnabled>", "NetworksEnabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeSourceSetting(t, root, tc.name, tc.body)
			if got := OrgShapeFeatures(root); !slices.Contains(got, tc.feature) {
				t.Fatalf("features=%v, want %s", got, tc.feature)
			}
			off := t.TempDir()
			writeSourceSetting(t, off, tc.name, strings.ReplaceAll(tc.body, "true", "false"))
			if got := OrgShapeFeatures(off); slices.Contains(got, tc.feature) {
				t.Fatalf("disabled setting produced %s: %v", tc.feature, got)
			}
		})
	}
}

// Settings are keyed by file name, merged per file, and read only when they
// are booleans in a settings folder. An unrelated key or non-boolean value
// establishes nothing, and the contributing files are cache inputs.
func TestSourceSettingsEvidenceBoundaries(t *testing.T) {
	root := t.TempDir()
	quote := writeSourceSetting(t, root, "Quote", "<enableQuote>true</enableQuote>")
	account := writeSourceSetting(t, root, "Account", "<enableRelateContactToMultipleAccounts>true</enableRelateContactToMultipleAccounts><showViewHierarchyLink>true</showViewHierarchyLink>")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/settings/Order.settings-meta.xml"), "<OrderSettings><enableOrders>true</enableOrders></OrderSettings>")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Thing__c/settings/Forecasting.settings-meta.xml"), "<ForecastingSettings><enableForecasts>true</enableForecasts></ForecastingSettings>")
	writeFile(t, filepath.Join(root, "node_modules/pkg/settings/Currency.settings-meta.xml"), "<CurrencySettings><enableMultiCurrency>true</enableMultiCurrency></CurrencySettings>")
	got := OrgShapeFeatures(root)
	if !reflect.DeepEqual(got, []string{"Quotes"}) {
		t.Fatalf("features=%v, want only Quotes (multiple accounts is not a trigger)", got)
	}
	inputs := OrgShapeInputFiles(root)
	for _, want := range []string{quote, account} {
		if !slices.Contains(inputs, want) {
			t.Errorf("settings file %s is not an org-shape cache input", want)
		}
	}
	for _, path := range inputs {
		if filepath.Base(path) == "Currency.settings-meta.xml" || filepath.Base(path) == "Forecasting.settings-meta.xml" {
			t.Errorf("skipped tree contributed %s", path)
		}
	}
}

func TestSourceSettingsWithScratchDefinitionUnion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Sites"],"settings":{"chatterSettings":{"enableChatter":true}}}`)
	writeSourceSetting(t, root, "Quote", "<enableQuote>true</enableQuote>")
	got := OrgShapeFeatures(root)
	for _, want := range []string{"Sites", "Chatter", "Quotes"} {
		if !slices.Contains(got, want) {
			t.Fatalf("features=%v, missing %s", got, want)
		}
	}
}
