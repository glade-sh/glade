package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

// Legacy named credential endpoints are queryable setup metadata, including
// from isolated tests. Modern endpoint storage is a separate contract.
func TestLegacyNamedCredentialQueryVisibleInIsolatedTest(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/namedCredentials/OwnedRelay.namedCredential-meta.xml"), `<NamedCredential xmlns="http://soap.sforce.com/2006/04/metadata"><label>Owned relay</label><endpoint>https://example.invalid/service</endpoint><principalType>Anonymous</principalType><protocol>NoAuthentication</protocol></NamedCredential>`)
			source := `@IsTest private class StoredEndpointSelection {
 @IsTest static void selectsEndpointWithoutRequest() {
  List<NamedCredential> rows=Database.query('SELECT Id,DeveloperName,Endpoint,NamespacePrefix FROM NamedCredential WHERE DeveloperName=\'OwnedRelay\' AND NamespacePrefix=\'\' WITH SYSTEM_MODE');
  System.assertEquals(1,rows.size());
  System.assertNotEquals(null,rows[0].Id);
  System.assertEquals('OwnedRelay',rows[0].DeveloperName);
  System.assertEquals('https://example.invalid/service',rows[0].Endpoint);
  System.assertEquals(null,rows[0].NamespacePrefix);
 }
 @IsTest static void missingMetadataStaysAbsent() {
  System.assertEquals(0,[SELECT COUNT() FROM NamedCredential WHERE DeveloperName='UnconfiguredRelay']);
 }
}`
			path := filepath.Join(root, "force-app/main/default/classes/StoredEndpointSelection.cls")
			writeFile(t, path, source)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"StoredEndpointSelection"}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
				encoded, _ := json.Marshal(run)
				t.Fatalf("legacy endpoint query: %s", encoded)
			}
		})
	}
}
