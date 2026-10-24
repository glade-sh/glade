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

func TestOrgShapeFeaturesDetectsPersonAccountsFromRetrievedRecordType(t *testing.T) {
	for name, layout := range map[string]string{
		"standard":     "force-app/main/default/objects/PersonAccount/recordTypes/Individual.recordType-meta.xml",
		"nested":       "App Name/force-app/main/default/objects/PersonAccount/recordTypes/Individual.recordType-meta.xml",
		"mdapi":        "src/objects/PersonAccount/recordTypes/Individual.recordType-meta.xml",
		"no-sfdx-root": "objects/PersonAccount/recordTypes/Individual.recordType-meta.xml",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, layout), `<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Individual</fullName></RecordType>`)
			got := OrgShapeFeatures(root)
			if len(got) != 1 || got[0] != "PersonAccounts" {
				t.Fatalf("features = %#v, want [PersonAccounts]", got)
			}
		})
	}
}

func TestOrgShapeFeaturesIgnoresPersonAccountLookalikes(t *testing.T) {
	root := t.TempDir()
	// An Account record type, a PersonAccount name elsewhere, and an empty object folder are not evidence.
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/recordTypes/PersonAccount.recordType-meta.xml"), `<RecordType/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/PersonAccount.cls"), `public class PersonAccount {}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/compactLayouts/L.compactLayout-meta.xml"), `<CompactLayout/>`)
	writeFile(t, filepath.Join(root, "node_modules/pkg/objects/PersonAccount/recordTypes/X.recordType-meta.xml"), `<RecordType/>`)
	if got := OrgShapeFeatures(root); len(got) != 0 {
		t.Fatalf("features = %#v, want none", got)
	}
}

func TestOrgShapeFeaturesDedupesDetectedPersonAccounts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"features": ["PersonAccounts"]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes/I.recordType-meta.xml"), `<RecordType/>`)
	got := OrgShapeFeatures(root)
	if len(got) != 1 || got[0] != "PersonAccounts" {
		t.Fatalf("features = %#v, want one PersonAccounts", got)
	}
}
