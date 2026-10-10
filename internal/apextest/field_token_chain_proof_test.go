package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// All four unchanged API53 FieldToken contracts passed Salesforce Wave86b.
func TestRunFieldTokenChainContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFieldTokenChain53.cls"), "@IsTest private class GladeFieldTokenChain53 {\n private static void checkToken(Schema.SObjectField value,Schema.SObjectField direct,String fieldName,Schema.SObjectType objectType){\n  System.assertNotEquals(null,value,'field token must exist');\n  System.assertEquals(direct,value,'field token identity');\n  Schema.DescribeFieldResult described=value.getDescribe();\n  System.assertEquals(fieldName,described.getName(),'field name');\n  System.assertEquals(objectType,described.getSObjectType(),'owning object');\n  System.assertEquals(value,described.getSObjectField(),'describe round trip');\n }\n @IsTest static void opportunityStaticChain(){\n  checkToken(SObjectType.Opportunity.fields.Amount.getSobjectField(),Opportunity.Amount,'Amount',Opportunity.SObjectType);\n }\n @IsTest static void contactRoleStaticChain(){\n  checkToken(SObjectType.OpportunityContactRole.fields.ContactId.getSobjectField(),OpportunityContactRole.ContactId,'ContactId',OpportunityContactRole.SObjectType);\n }\n @IsTest static void customCurrencyStaticChain(){\n  checkToken(SObjectType.GladeDescribeProbe53__c.fields.Amount__c.getSobjectField(),GladeDescribeProbe53__c.Amount__c,'Amount__c',GladeDescribeProbe53__c.SObjectType);\n }\n @IsTest static void explicitDescribeControls(){\n  checkToken(Opportunity.Amount.getDescribe().getSObjectField(),Opportunity.Amount,'Amount',Opportunity.SObjectType);\n  checkToken(OpportunityContactRole.ContactId.getDescribe().getSObjectField(),OpportunityContactRole.ContactId,'ContactId',OpportunityContactRole.SObjectType);\n  checkToken(GladeDescribeProbe53__c.Amount__c.getDescribe().getSObjectField(),GladeDescribeProbe53__c.Amount__c,'Amount__c',GladeDescribeProbe53__c.SObjectType);\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeFieldTokenChain53.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDescribeProbe53__c/GladeDescribeProbe53__c.object-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomObject xmlns=\"http://soap.sforce.com/2006/04/metadata\"><deploymentStatus>Deployed</deploymentStatus><label>Owned Describe Probe</label><nameField><label>Owned Describe Name</label><type>Text</type></nameField><pluralLabel>Owned Describe Probes</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeDescribeProbe53__c/fields/Amount__c.field-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<CustomField xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fullName>Amount__c</fullName><label>Owned Describe Amount</label><precision>18</precision><scale>2</scale><type>Currency</type></CustomField>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("field token chain contracts: %s", data)
	}
}
