package project

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestOrgShapeFeaturesLoadsStringCumulusCIConfig(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n")
	writeFile(t, filepath.Join(root, "orgs/dev.json"), `{"orgName":"Owned Dev Workspace","edition":"Developer","hasSampleData":"false","features":"Sites","settings":{"chatterSettings":{"enableChatter":true}}}`)
	got := OrgShapeFeatures(root)
	if len(got) != 2 || got[0] != "Sites" || got[1] != "Chatter" {
		t.Fatalf("features=%v", got)
	}
}

func TestScratchFeatureNamesRejectsNonStrings(t *testing.T) {
	for _, input := range []string{`42`, `true`, `{}`, `["Sites",2]`} {
		var def scratchOrgDefinition
		if err := json.Unmarshal([]byte(`{"features":`+input+`}`), &def); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	for _, input := range []string{`null`, `["Sites"]`, `"Sites"`} {
		var def scratchOrgDefinition
		if err := json.Unmarshal([]byte(`{"features":`+input+`}`), &def); err != nil {
			t.Fatalf("%s: %v", input, err)
		}
	}
}

func TestScratchSettingsCurrencyTrigger(t *testing.T) {
	for _, value := range []any{true, false, "true", nil} {
		got := scratchSettingsFeatures(map[string]any{"currencySettings": map[string]any{"enableMultiCurrency": value}})
		want := value == true
		found := false
		for _, feature := range got {
			found = found || feature == "MultiCurrency"
		}
		if found != want {
			t.Errorf("enableMultiCurrency=%v: features=%v, want enabled=%v", value, got, want)
		}
	}
}

func TestScratchSettingsUnrelatedUserManagement(t *testing.T) {
	for _, settings := range []any{map[string]any{}, map[string]any{"enableNewUserEmailVerification": true}, map[string]any{"unrecognizedSetting": true}, false} {
		for _, feature := range scratchSettingsFeatures(map[string]any{"userManagementSettings": settings}) {
			if feature == "EnableSetPasswordInApi" {
				t.Errorf("unrelated userManagementSettings %v enables password API", settings)
			}
		}
	}
}

func TestScratchSettingsQuoteTrigger(t *testing.T) {
	for _, value := range []bool{false, true} {
		got := scratchSettingsFeatures(map[string]any{"quoteSettings": map[string]any{"enableQuote": value}})
		found := false
		for _, feature := range got {
			found = found || feature == "Quotes"
		}
		if found != value {
			t.Errorf("enableQuote=%t: features=%v", value, got)
		}
	}
}
