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
