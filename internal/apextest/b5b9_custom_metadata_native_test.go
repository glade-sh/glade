package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// b5b9_feature_objects/org.tsv records describe membership, the missing-field
// control, and the queried MasterLabel/Label values on APIs 62/67. The owned
// __mdt object and record are exactly the synthetic native inputs.
func TestB5B9CustomMetadataMasterLabelNativeRoutes(t *testing.T) {
	const fixture = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/force-app/main/default/"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
			for _, name := range []string{"objects/OwnedFlag__mdt/OwnedFlag__mdt.object-meta.xml", "customMetadata/OwnedFlag.Example.md-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(fixture, name))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default", name), string(data))
			}
			body := `System.assertEquals(true,OwnedFlag__mdt.SObjectType.getDescribe().fields.getMap().containsKey('MasterLabel'));` +
				`System.assertEquals(false,OwnedFlag__mdt.SObjectType.getDescribe().fields.getMap().containsKey('OwnedMissing__c'));` +
				`OwnedFlag__mdt row=[SELECT MasterLabel,Label FROM OwnedFlag__mdt WHERE DeveloperName='Example' LIMIT 1];` +
				`System.assertEquals('Owned example',row.MasterLabel);System.assertEquals('Owned example',row.Label);`
			runStandardFieldEvidenceRoutes(t, root, api, body)
		})
	}
}
