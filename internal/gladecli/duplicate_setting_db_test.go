package gladecli

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDBStandardDuplicateSettingFollowsProject(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Name__c")
	db := filepath.Join(root, "org.sqlite")
	for _, tc := range []struct {
		value string
		want  bool
	}{{"on", true}, {"off", false}, {"on", true}} {
		writeTestFile(t, filepath.Join(root, "glade.yml"), "duplicateRules:\n  standard: "+tc.value+"\n")
		store, org, err := openDBStore(db, root)
		if err != nil {
			t.Fatal(err)
		}
		if org.StandardDuplicateRules != tc.want {
			t.Errorf("%s: got %v", tc.value, org.StandardDuplicateRules)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDBStandardDuplicateProjectMetadataWinsAfterReload(t *testing.T) {
	root := t.TempDir()
	writeProjectWithWidgetField(t, root, "Name__c")
	writeTestFile(t, filepath.Join(root, "glade.yml"), "duplicateRules:\n  standard: off\n")
	writeTestFile(t, filepath.Join(root, "force-app/main/default/duplicateRules/Account.Standard_Account_Duplicate_Rule.duplicateRule-meta.xml"), `<DuplicateRule xmlns="http://soap.sforce.com/2006/04/metadata"><isActive>true</isActive><actionOnInsert>Block</actionOnInsert><masterLabel>Project Rule</masterLabel></DuplicateRule>`)
	db := filepath.Join(root, "org.sqlite")
	for i := 0; i < 3; i++ {
		store, org, err := openDBStore(db, root)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, rule := range org.Objects["Account"].Definition.DuplicateRules {
			if rule.Name == "Standard_Account_Duplicate_Rule" {
				found = true
				if rule.Seeded || !rule.Active || rule.ActionOnInsert != "Block" {
					t.Errorf("open %d: project rule = %+v", i, rule)
				}
			}
		}
		if !found {
			t.Errorf("open %d: project rule missing", i)
		}
		if i == 1 {
			legacy := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&legacy)
			state := org.Objects["Account"]
			state.Definition.DuplicateRules = legacy.Objects["Account"].Definition.DuplicateRules
			org.Objects["Account"] = state
			if err := store.Save(org); err != nil {
				t.Fatal(err)
			}
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			raw, err := sql.Open("sqlite", db)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := raw.Exec(`update object_definitions set definition_json = replace(cast(definition_json as text), '"seeded":true,', '')`); err != nil {
				t.Fatal(err)
			}
			if _, err := raw.Exec(`delete from org_meta where key = ?`, "glade.org.duplicateRuleOrigins"); err != nil {
				t.Fatal(err)
			}
			if err := raw.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
