package vm

import (
	"testing"
)

func TestCB170NewDMLOptionsOptAllOrNoneDefaultsToNull(t *testing.T) {
	program, err := CompileAnonymous(`
Database.DMLOptions opts = new Database.DMLOptions();
System.assertEquals('DMLOptions:[AllowFieldTruncation=null, AssignmentRuleHeader=null, DuplicateRuleHeader=null, EmailHeader=null, LocaleOptions=null, LocalizeErrors=null, OptAllOrNone=null]', String.valueOf(opts), 'Salesforce DMLOptions display');
System.assertEquals('AssignmentRuleHeader:[AssignmentRuleId=null, UseDefaultRule=null]', String.valueOf(opts.AssignmentRuleHeader), 'Salesforce AssignmentRuleHeader display');
System.assertEquals('DuplicateRuleHeader:[AllowSave=null, RunAsCurrentUser=null]', String.valueOf(opts.DuplicateRuleHeader), 'Salesforce DuplicateRuleHeader display');
System.assertEquals('EmailHeader:[TriggerAutoResponseEmail=null, TriggerOtherEmail=null, TriggerUserEmail=null]', String.valueOf(opts.EmailHeader), 'Salesforce EmailHeader display');
System.assertEquals(null, opts.optAllOrNone, 'default optAllOrNone lower');
System.assertEquals(null, opts.OptAllOrNone, 'default OptAllOrNone upper');
System.assertEquals(null, opts.allowFieldTruncation, 'default allowFieldTruncation lower');
System.assertEquals(null, opts.AllowFieldTruncation, 'default AllowFieldTruncation upper');
System.assertEquals(null, opts.localizeErrors, 'default localizeErrors lower');
System.assertEquals(null, opts.LocalizeErrors, 'default LocalizeErrors upper');
System.assertEquals(null, opts.AssignmentRuleHeader.AssignmentRuleId, 'default assignment rule id');
System.assertEquals(null, opts.AssignmentRuleHeader.UseDefaultRule, 'default assignment rule flag');
System.assertEquals(null, opts.DuplicateRuleHeader.AllowSave, 'default duplicate rule flag');
System.assertEquals(null, opts.DuplicateRuleHeader.RunAsCurrentUser, 'default duplicate user flag');
System.assertEquals(null, opts.EmailHeader.TriggerAutoResponseEmail, 'default auto response flag');
System.assertEquals(null, opts.EmailHeader.TriggerOtherEmail, 'default other email flag');
System.assertEquals(null, opts.EmailHeader.TriggerUserEmail, 'default user email flag');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB236DatabaseDTODisplayContracts(t *testing.T) {
	program, err := CompileAnonymous(`
Database.LeadConvert convert = new Database.LeadConvert();
System.assertEquals('Database.LeadConvert[getAccountId=null;getAccountRecord=null;getBypassAccountDedupeCheck=null;getBypassContactDedupeCheck=null;getContactId=null;getContactRecord=null;getConvertedStatus=null;getLeadId=null;getOpportunityId=null;getOpportunityName=null;getOpportunityRecord=null;getOwnerId=null;getRelatedPersonAccountId=null;getRelatedPersonAccountRecord=null;isDoNotCreateOpportunity=false;isOverwriteLeadSource=false;isSendNotificationEmail=false;]', String.valueOf(convert), 'Salesforce LeadConvert display');
Database.QueryLocator locator = Database.getQueryLocator('SELECT Id FROM Account LIMIT 1');
System.assertEquals('Database.QueryLocator[Query=SELECT Id FROM Account LIMIT 1]', String.valueOf(locator), 'Salesforce QueryLocator display');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
