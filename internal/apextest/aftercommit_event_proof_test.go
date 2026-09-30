package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact API66 callback and direct publish assertions admitted by Salesforce SF166.
func TestAfterCommitEventDMLAPI66Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeAfterCommit66Proof.cls"), "@IsTest private class GladeAfterCommit66Proof {\n private class Callback implements Metadata.DeployCallback {\n  public void handleResult(Metadata.DeployResult result, Metadata.DeployCallbackContext context) {\n   EventBus.publish(new UserNotification__e(Type__c='DeploymentResult', Payload__c=JSON.serialize(result)));\n  }\n }\n @IsTest static void callbackPublishChargesDml() {\n  Metadata.DeployResult result = new Metadata.DeployResult();\n  result.status = Metadata.DeployStatus.Succeeded;\n  Integer statementsBefore = Limits.getDmlStatements();\n  Integer rowsBefore = Limits.getDmlRows();\n  new Callback().handleResult(result, new Metadata.DeployCallbackContext());\n  System.assertEquals(1, Limits.getDmlStatements()-statementsBefore);\n  System.assertEquals(1, Limits.getDmlRows()-rowsBefore);\n }\n @IsTest static void directPublishChargesDml() {\n  Integer statementsBefore = Limits.getDmlStatements();\n  Integer rowsBefore = Limits.getDmlRows();\n  Database.SaveResult result = EventBus.publish(new UserNotification__e(Type__c='DeploymentResult', Payload__c='{}'));\n  System.assertEquals(true,result.isSuccess());\n  System.assertEquals(1, Limits.getDmlStatements()-statementsBefore);\n  System.assertEquals(1, Limits.getDmlRows()-rowsBefore);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeAfterCommit66Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>66.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/UserNotification__e/UserNotification__e.object-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <deploymentStatus>Deployed</deploymentStatus>\n    <description>Channel to send updates to users from async actions</description>\n    <eventType>HighVolume</eventType>\n    <label>User Notification</label>\n    <pluralLabel>User Notifications</pluralLabel>\n    <publishBehavior>PublishAfterCommit</publishBehavior>\n</CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/UserNotification__e/fields/Type__c.field-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Type__c</fullName>\n    <description>Categorical type, one of the following\nDeploymentResult</description>\n    <externalId>false</externalId>\n    <isFilteringDisabled>false</isFilteringDisabled>\n    <isNameField>false</isNameField>\n    <isSortingDisabled>false</isSortingDisabled>\n    <label>Type</label>\n    <length>255</length>\n    <required>true</required>\n    <type>Text</type>\n    <unique>false</unique>\n</CustomField>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/UserNotification__e/fields/Payload__c.field-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n    <fullName>Payload__c</fullName>\n    <description>JSON Payload Body</description>\n    <externalId>false</externalId>\n    <isFilteringDisabled>false</isFilteringDisabled>\n    <isNameField>false</isNameField>\n    <isSortingDisabled>false</isSortingDisabled>\n    <label>Payload</label>\n    <length>32768</length>\n    <type>LongTextArea</type>\n    <visibleLines>3</visibleLines>\n</CustomField>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"namespace\": \"\", \"sourceApiVersion\": \"66.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, _ := json.Marshal(run)
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("after-commit proof: %s", data)
	}
}
