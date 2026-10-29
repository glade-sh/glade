package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// A project declaration can rename the standard person record type. A label
// reference elsewhere in the project must denote that same record type.
func TestPersonAccountDeclaredLabelLookupRegression(t *testing.T) {
	for _, label := range []string{"Member", "Client", "Resident"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(label+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"orgName":"Synthetic","edition":"Developer","features":["PersonAccounts"]}`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/PersonAccount.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Person Account</label><pluralLabel>Person Accounts</pluralLabel></CustomObject>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes/PersonAccount.recordType-meta.xml"), fmt.Sprintf(`<RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>PersonAccount</fullName><active>true</active><label>%s</label></RecordType>`, label))
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Probe__c/Probe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Probe</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Probes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>`)
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Probe__c/fields/Account__c.field-meta.xml"), fmt.Sprintf(`<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Account__c</fullName><label>Account</label><lookupFilter><active>true</active><filterItems><field>Account.RecordTypeId</field><operation>equals</operation><value>%s</value></filterItems><isOptional>false</isOptional></lookupFilter><referenceTo>Account</referenceTo><relationshipLabel>Probes</relationshipLabel><relationshipName>Probes</relationshipName><type>Lookup</type></CustomField>`, label))
				class := fmt.Sprintf(`@IsTest private class LabelLookupTest {
  @IsTest static void check() {
    Id byName = Schema.SObjectType.Account.getRecordTypeInfosByName().get('%s').getRecordTypeId();
    Id byDeveloper = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
    Account person = new Account(LastName='Example', RecordTypeId=byDeveloper);
    insert person;
    insert new Probe__c(Name='example', Account__c=person.Id);
    System.assertEquals(byDeveloper, byName);
  }
}`, label)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelLookupTest.cls"), class)
				writeFile(t, filepath.Join(root, "force-app/main/default/classes/LabelLookupTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem != nil {
								t.Errorf("%s: %s", c.MethodName, c.Problem.Message)
							}
						}
					}
					t.Fatalf("summary=%+v", got)
				}
			})
		}
	}
}
