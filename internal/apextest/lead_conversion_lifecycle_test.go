package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLeadConversionSF237(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeLeadConversion65Capture.cls"), `public class GladeLeadConversion65Capture {
 public static Id target;
 public static Boolean sawTransition=false;
 public static Id accountId,contactId,opportunityId;
 public static void capture(List<Lead> rows,Map<Id,Lead> oldRows){
  for(Lead row:rows){
   if(row.Id==target && row.IsConverted && !oldRows.get(row.Id).IsConverted){
    sawTransition=true; accountId=row.ConvertedAccountId; contactId=row.ConvertedContactId; opportunityId=row.ConvertedOpportunityId;
    update new Account(Id=accountId,Description='Owned conversion after update');
    update new Contact(Id=contactId,Description='Owned conversion after update');
    if(opportunityId!=null) update new Opportunity(Id=opportunityId,Description='Owned conversion after update');
   }
  }
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeLeadConversion65Capture.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeLeadConversion65Assertions.cls"), `@IsTest private class GladeLeadConversion65Assertions {
 @IsTest static void withoutOpportunity(){checkConversion(true);}
 @IsTest static void withOpportunity(){checkConversion(false);}
 private static void checkConversion(Boolean skipOpportunity){
  Lead row=new Lead(LastName='Owned conversion',Company='Owned conversion company'); insert row;
  GladeLeadConversion65Capture.target=row.Id;
  Database.LeadConvert lc=new Database.LeadConvert();lc.setLeadId(row.Id);lc.setDoNotCreateOpportunity(skipOpportunity);
  lc.setConvertedStatus([SELECT MasterLabel FROM LeadStatus WHERE IsConverted=TRUE LIMIT 1].MasterLabel);
  Database.LeadConvertResult converted=Database.convertLead(lc);
  System.assertEquals(true,converted.isSuccess());
  System.assertEquals(true,GladeLeadConversion65Capture.sawTransition);
  System.assertEquals(converted.getAccountId(),GladeLeadConversion65Capture.accountId);
  System.assertEquals(converted.getContactId(),GladeLeadConversion65Capture.contactId);
  System.assertEquals(converted.getOpportunityId(),GladeLeadConversion65Capture.opportunityId);
  System.assertEquals('Owned conversion after update',[SELECT Description FROM Account WHERE Id=:converted.getAccountId()].Description);
  System.assertEquals('Owned conversion after update',[SELECT Description FROM Contact WHERE Id=:converted.getContactId()].Description);
  if(skipOpportunity) System.assertEquals(null,converted.getOpportunityId());
  else System.assertEquals('Owned conversion after update',[SELECT Description FROM Opportunity WHERE Id=:converted.getOpportunityId()].Description);
  Lead stored=[SELECT IsConverted,ConvertedAccountId,ConvertedContactId,ConvertedOpportunityId FROM Lead WHERE Id=:row.Id];
  System.assertEquals(true,stored.IsConverted);System.assertEquals(converted.getAccountId(),stored.ConvertedAccountId);System.assertEquals(converted.getContactId(),stored.ConvertedContactId);System.assertEquals(converted.getOpportunityId(),stored.ConvertedOpportunityId);
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeLeadConversion65Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/GladeLeadConversion65Trigger.trigger"), `trigger GladeLeadConversion65Trigger on Lead (after update) { GladeLeadConversion65Capture.capture(Trigger.new,Trigger.oldMap); }`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/GladeLeadConversion65Trigger.trigger-meta.xml"), `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexTrigger>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		b, _ := json.Marshal(run)
		t.Fatalf("Lead conversion: %s", b)
	}
}

// Existing conversion transaction semantics must also cover newly dispatched
// trigger failures, both default all-or-none and partial-result calls.
func TestLeadConversionAfterUpdateFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"65.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/triggers/OwnedLeadReject.trigger"), `trigger OwnedLeadReject on Lead (after update) { for(Lead row:Trigger.new) if(row.IsConverted && !Trigger.oldMap.get(row.Id).IsConverted) row.addError('Owned conversion rejection'); }`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedLeadRollback.cls"), `@IsTest private class OwnedLeadRollback {
 @IsTest static void defaultAndPartial(){
  Lead row=new Lead(LastName='Owned rollback',Company='Owned rollback company');insert row;
  Database.LeadConvert conversion=new Database.LeadConvert();conversion.setLeadId(row.Id);conversion.setDoNotCreateOpportunity(true);
  conversion.setConvertedStatus([SELECT MasterLabel FROM LeadStatus WHERE IsConverted=TRUE LIMIT 1].MasterLabel);
  Boolean caught=false;try {Database.convertLead(conversion);}catch(DmlException ex){caught=true;}
  System.assertEquals(true,caught);
  System.assertEquals(0,[SELECT COUNT() FROM Account]);System.assertEquals(0,[SELECT COUNT() FROM Contact]);
  System.assertEquals(false,[SELECT IsConverted FROM Lead WHERE Id=:row.Id].IsConverted);
  Database.LeadConvertResult result=Database.convertLead(conversion,false);System.assertEquals(false,result.isSuccess());
  System.assertEquals(0,[SELECT COUNT() FROM Account]);System.assertEquals(0,[SELECT COUNT() FROM Contact]);
  System.assertEquals(false,[SELECT IsConverted FROM Lead WHERE Id=:row.Id].IsConverted);
 }
}`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("Lead rollback: %s", b)
	}
}
