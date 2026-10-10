package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// All three unchanged API53 RecordType projection contracts passed Salesforce Wave87.
func TestRunRecordTypeProjectionContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRecordTypeProjection53.cls"), "@IsTest private class GladeRecordTypeProjection53 {\n private static void checkRow(RecordType row){\n  System.assertNotEquals(null,row.Id,'queried metadata ID');\n  System.assertEquals('Owned_Probe',row.DeveloperName,'developer name');\n  System.assertEquals('Owned Probe Label',row.Name,'projected label');\n  Map<String,Schema.RecordTypeInfo> byName=GladeDescribeProbe53__c.SObjectType.getDescribe().getRecordTypeInfosByName();\n  System.assert(byName.containsKey(row.Name),'queried label joins describe map');\n  Schema.RecordTypeInfo info=byName.get(row.Name);\n  System.assert(info.isActive(),'owned record type is active');\n  System.assertEquals(row.Id,info.getRecordTypeId(),'queried ID joins describe ID');\n  System.assertEquals(row.Name,info.getName(),'describe label');\n  System.assertEquals(row.DeveloperName,info.getDeveloperName(),'describe developer name');\n  System.assertEquals(row.Name,GladeDescribeProbe53__c.SObjectType.getDescribe().getRecordTypeInfosById().get(row.Id).getName(),'ID map joins label');\n }\n @IsTest static void plainNameControl(){\n  List<RecordType> rows=[SELECT Id,Name,DeveloperName FROM RecordType WHERE SobjectType='GladeDescribeProbe53__c' AND DeveloperName='Owned_Probe' AND IsActive=true];\n  System.assertEquals(1,rows.size(),'exact owned population');checkRow(rows[0]);\n }\n @IsTest static void unqualifiedLabel(){\n  List<RecordType> rows=[SELECT Id,toLabel(Name),DeveloperName FROM RecordType WHERE SobjectType='GladeDescribeProbe53__c' AND DeveloperName='Owned_Probe' AND IsActive=true];\n  System.assertEquals(1,rows.size(),'exact owned population');checkRow(rows[0]);\n }\n @IsTest static void originalQualifiedLabel(){\n  List<RecordType> rows=[SELECT Id,toLabel(Recordtype.Name),DeveloperName FROM RecordType WHERE SobjectType='GladeDescribeProbe53__c' AND DeveloperName='Owned_Probe' AND IsActive=true ORDER BY DeveloperName ASC LIMIT 1];\n  System.assertEquals(1,rows.size(),'exact owned population');checkRow(rows[0]);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeRecordTypeProjection53.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDescribeProbe53__c/GladeDescribeProbe53__c.object-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Owned Describe Probe</label><nameField><label>Owned Describe Name</label><type>Text</type></nameField><pluralLabel>Owned Describe Probes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDescribeProbe53__c/recordTypes/Owned_Probe.recordType-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<RecordType xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Owned_Probe</fullName><active>true</active><label>Owned Probe Label</label></RecordType>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("record type projection contracts failed: %s", data)
	}
}
