package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultCustomAccessPermissiveDefault(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := "testdata/conformance/default_custom_access/project"
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			for _, metadataDir := range []string{"profiles", "permissionsets"} {
				if _, err := os.Stat(filepath.Join(root, "force-app/main/default", metadataDir)); !os.IsNotExist(err) {
					t.Fatalf("fixture must not define %s metadata: %v", metadataDir, err)
				}
			}

			class := `@IsTest private class DefaultAccessPolicyProbe {
 @IsTest static void keepsDefaultAccessWithoutProfileMetadata() {
  Schema.DescribeSObjectResult objectInfo=AccessNote__c.SObjectType.getDescribe();
  Schema.DescribeFieldResult fieldInfo=AccessNote__c.Detail__c.getDescribe();
  System.assert(objectInfo.isAccessible(),'default custom-object read access');
  System.assert(objectInfo.isCreateable(),'default custom-object create access');
  System.assert(objectInfo.isUpdateable(),'default custom-object update access');
  System.assert(objectInfo.isDeletable(),'default custom-object delete access');
  System.assert(fieldInfo.isAccessible(),'default custom-field read access');
  System.assert(fieldInfo.isCreateable(),'default custom-field create access');
  System.assert(fieldInfo.isUpdateable(),'default custom-field update access');
  insert new AccessNote__c(Name='Default access',Detail__c='local');
  Database.insert(new AccessNote__c(Name='User mode access',Detail__c='user-mode'),AccessLevel.USER_MODE);
  System.assertEquals('local',[SELECT Detail__c FROM AccessNote__c WHERE Name='Default access' WITH USER_MODE LIMIT 1].Detail__c);
  System.assertEquals(2,[SELECT COUNT() FROM AccessNote__c WITH USER_MODE]);
 }
}`
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DefaultAccessPolicyProbe.cls"), class)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/DefaultAccessPolicyProbe.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?><ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")

			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatalf("fixture did not load: %v", index.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"DefaultAccessPolicyProbe"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				data, _ := json.Marshal(run)
				t.Fatalf("permissive default without profile metadata: summary=%+v run=%s", summary, data)
			}
		})
	}
}
