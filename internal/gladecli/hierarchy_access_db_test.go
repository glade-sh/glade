package gladecli

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestHierarchyAccessDBConfigTransitions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeTestFile(t, filepath.Join(root, "force-app/main/default/objects/PrivateProbe__c/PrivateProbe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Private Probe</label><pluralLabel>Private Probes</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>Private</sharingModel></CustomObject>`)
			db := filepath.Join(root, "local.sqlite")
			for _, step := range []struct {
				config string
				want   string
			}{
				{"org:\n  hierarchyAccessObjects: []\n", ""},
				{"org:\n  hierarchyAccessObjects: [PrivateProbe__c]\n", "true"},
				{"org:\n  hierarchyAccessObjects: []\n", ""},
			} {
				writeTestFile(t, filepath.Join(root, "glade.yml"), step.config)
				store, org, err := openDBStore(db, root)
				if err != nil {
					t.Fatal(err)
				}
				got := org.Objects["PrivateProbe__c"].Definition.Metadata["gladeHierarchyAccess"]
				if err := store.Save(org); err != nil {
					t.Fatal(err)
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				if got != step.want {
					t.Fatalf("hierarchy setting = %q, want %q", got, step.want)
				}
			}
		})
	}
}
