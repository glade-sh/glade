package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB5LeadSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB5LeadSource.cls"), "@IsTest private class GladeB5LeadSource {\n private static void convertAndAssert(Boolean overwrite, String expected) {\n  Account account=new Account(Name='Glade owned conversion'); insert account;\n  Contact contact=new Contact(LastName='Glade contact',AccountId=account.Id,LeadSource='Web'); insert contact;\n  Lead lead=new Lead(LastName='Glade lead',Company='Glade owned conversion',LeadSource='Phone Inquiry'); insert lead;\n  LeadStatus converted=[SELECT MasterLabel FROM LeadStatus WHERE IsConverted=true LIMIT 1];\n  Database.LeadConvert request=new Database.LeadConvert();\n  request.setLeadId(lead.Id); request.setAccountId(account.Id); request.setContactId(contact.Id);\n  request.setConvertedStatus(converted.MasterLabel); request.setDoNotCreateOpportunity(true); request.setOverwriteLeadSource(overwrite);\n  Database.LeadConvertResult result=Database.convertLead(request);\n  System.assertEquals(true,result.isSuccess()); System.assertEquals(contact.Id,result.getContactId());\n  System.assertEquals(account.Id,result.getAccountId()); System.assertEquals(null,result.getOpportunityId());\n  Contact loaded=[SELECT LeadSource,AccountId FROM Contact WHERE Id=:contact.Id];\n  System.assertEquals(expected,loaded.LeadSource); System.assertEquals(account.Id,loaded.AccountId);\n  Lead changed=[SELECT IsConverted,ConvertedContactId,ConvertedAccountId FROM Lead WHERE Id=:lead.Id];\n  System.assertEquals(true,changed.IsConverted); System.assertEquals(contact.Id,changed.ConvertedContactId); System.assertEquals(account.Id,changed.ConvertedAccountId);\n }\n @IsTest static void overwriteCopiesLeadSource() { convertAndAssert(true,'Phone Inquiry'); }\n @IsTest static void noOverwritePreservesContactSource() { convertAndAssert(false,'Web'); }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB5LeadSource.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b5-lead-source-overwrite-api67\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeB5NeutralOptions(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB5NeutralOptions.cls"), "@IsTest private class GladeB5NeutralOptions {\n @IsTest static void explicitFalseAllowsInsert() {\n  Database.DMLOptions options=new Database.DMLOptions(); options.LocalizeErrors=false;\n  Account account=new Account(Name='Glade neutral option'); account.setOptions(options);\n  Database.SaveResult result=Database.insert(account);\n  System.assertEquals(true,result.isSuccess()); System.assertNotEquals(null,result.getId());\n  Account loaded=[SELECT Name FROM Account WHERE Id=:result.getId()]; System.assertEquals('Glade neutral option',loaded.Name);\n }\n @IsTest static void explicitFalseRetainsValidationError() {\n  Database.DMLOptions options=new Database.DMLOptions(); options.LocalizeErrors=false;\n  Account account=new Account(); account.setOptions(options);\n  Database.SaveResult result=Database.insert(account,false);\n  System.assertEquals(false,result.isSuccess()); System.assertEquals(null,result.getId());\n  System.assertEquals(1,result.getErrors().size()); System.assertEquals(StatusCode.REQUIRED_FIELD_MISSING,result.getErrors()[0].getStatusCode());\n  System.assertEquals(true,result.getErrors()[0].getFields().contains('Name'));\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeB5NeutralOptions.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"b5-neutral-dml-options-api67\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
