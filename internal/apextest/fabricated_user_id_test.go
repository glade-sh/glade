package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Test code commonly fabricates Ids for records it never inserts by padding
// a counter after the key prefix. Org user Ids are not sequential, so a
// fabricated User Id queries no user, including platform users.
func TestFabricatedSequentialUserIdMatchesNoSeededPlatformUser(t *testing.T) {
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n"
	writeFile(t, filepath.Join(classes, "PaddedIdMaker.cls"), `public class PaddedIdMaker {
 private static Integer counter = 0;
 public static Id next(Schema.SObjectType token) {
  counter++;
  String digits = String.valueOf(counter);
  return Id.valueOf(token.getDescribe().getKeyPrefix() + '000000000000'.substring(digits.length()) + digits);
 }
}
`)
	writeFile(t, filepath.Join(classes, "PaddedIdMaker.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(classes, "PaddedIdMakerTest.cls"), `@IsTest private class PaddedIdMakerTest {
 @IsTest static void fabricatedUserIdQueriesNoUser() {
  Id parentId = PaddedIdMaker.next(Account.SObjectType);
  Id ownerId = PaddedIdMaker.next(User.SObjectType);
  System.assertEquals('005000000000002', String.valueOf(ownerId).left(15));
  System.assertEquals(0, [SELECT COUNT() FROM User WHERE Id = :ownerId]);
  System.assertEquals(0, [SELECT COUNT() FROM User WHERE Id IN :new Set<String>{ String.valueOf(ownerId) }]);
 }
}
`)
	writeFile(t, filepath.Join(classes, "PaddedIdMakerTest.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"namespace\": \"\", \"sourceApiVersion\": \"66.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, _ := json.Marshal(run)
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("fabricated user id: %s", data)
	}
}
