package project

import (
	"path/filepath"
	"testing"
)

func TestLoadOwnerSharingMetadataAcrossOverlappingSourceRoots(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true},{"path":"force-app/main"}]}`)
	modern := filepath.Join(root, "force-app/main/default/sharingRules/ZBox__c.sharingRules-meta.xml")
	legacy := filepath.Join(root, "force-app/main/default/sharingRules/ABox__c.sharingRules")
	writeFile(t, modern, `<SharingRules/>`)
	writeFile(t, legacy, `<SharingRules/>`)
	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.OwnerSharingRuleFiles) != 2 || p.OwnerSharingRuleFiles[0] != legacy || p.OwnerSharingRuleFiles[1] != modern {
		t.Fatalf("owner sharing discovery/order/deduplication: %v", p.OwnerSharingRuleFiles)
	}
}
