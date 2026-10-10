package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestRunGeneratedShareFieldTokensAPI67(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTier0SalesforceE175Proof.cls"), `@IsTest private class GladeTier0SalesforceE175Proof {
 private static final Map<SObjectField, Set<String>> FIELDS = new Map<SObjectField, Set<String>> {
  Schema.GladeTier0__Share.AccessLevel => new Set<String>(),
  Schema.GladeTier0__Share.RowCause => new Set<String>()
 };
 @IsTest static void generatedShareFieldTokens() {
  Schema.SObjectField accessLevel = Schema.GladeTier0__Share.AccessLevel;
  Schema.SObjectField rowCause = Schema.GladeTier0__Share.RowCause;
  System.assertEquals('AccessLevel', accessLevel.getDescribe().getName());
  System.assertEquals('RowCause', rowCause.getDescribe().getName());
  System.assertEquals(2, FIELDS.size());
  System.assert(FIELDS.containsKey(accessLevel));
  System.assert(FIELDS.containsKey(rowCause));
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTier0SalesforceE175Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeTier0__c/GladeTier0__c.object-meta.xml"), "<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><enableSharing>true</enableSharing><label>Glade Tier 0</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Glade Tier 0s</pluralLabel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}\n")

	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("generated share field tokens: %s", data)
	}
}
