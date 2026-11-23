@IsTest
private class R5RtProbe {
  @IsTest static void x_rt() {
    List<String> r5out = new List<String>();
try {
  Id rt = Schema.SObjectType.X__c.getRecordTypeInfosByDeveloperName().get('RT').getRecordTypeId();
  X__c x = new X__c(Name='a', RecordTypeId=rt); insert x;
  r5out.add('R5|x_rt|P=' + [SELECT P__c FROM X__c WHERE Id=:x.Id].P__c + '|inMemory=' + x.P__c);
} catch (Exception e) { r5out.add('R5|x_rt|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
  @IsTest static void x_none() {
    List<String> r5out = new List<String>();
try {
  X__c x = new X__c(Name='a'); insert x;
  r5out.add('R5|x_none|P=' + [SELECT P__c FROM X__c WHERE Id=:x.Id].P__c);
} catch (Exception e) { r5out.add('R5|x_none|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
  @IsTest static void acct_rt() {
    List<String> r5out = new List<String>();
try {
  Id rt = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('RT').getRecordTypeId();
  Account a = new Account(Name='a', RecordTypeId=rt); insert a;
  r5out.add('R5|acct_rt|Industry=' + [SELECT Industry FROM Account WHERE Id=:a.Id].Industry);
} catch (Exception e) { r5out.add('R5|acct_rt|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
  @IsTest static void bla_rt() {
    List<String> r5out = new List<String>();
try {
  RegulatoryAuthorizationType t = new RegulatoryAuthorizationType(Name='t', RegulatoryAuthCategory='License'); insert t;
  Id rt = Schema.SObjectType.BusinessLicenseApplication.getRecordTypeInfosByDeveloperName().get('RT').getRecordTypeId();
  BusinessLicenseApplication b = new BusinessLicenseApplication(RecordTypeId=rt, LicenseTypeId=t.Id, AppliedDate=DateTime.now()); insert b;
  r5out.add('R5|bla_rt|Category=' + [SELECT Category FROM BusinessLicenseApplication WHERE Id=:b.Id].Category);
} catch (Exception e) { r5out.add('R5|bla_rt|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
  @IsTest static void bla_nort() {
    List<String> r5out = new List<String>();
try {
  RegulatoryAuthorizationType t = new RegulatoryAuthorizationType(Name='t2', RegulatoryAuthCategory='License'); insert t;
  BusinessLicenseApplication b = new BusinessLicenseApplication(LicenseTypeId=t.Id, AppliedDate=DateTime.now()); insert b;
  r5out.add('R5|bla_nort|Category=' + [SELECT Category FROM BusinessLicenseApplication WHERE Id=:b.Id].Category);
} catch (Exception e) { r5out.add('R5|bla_nort|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
  @IsTest static void uom() {
    List<String> r5out = new List<String>();
try {
  UnitOfMeasure u = new UnitOfMeasure(Name='Milligrams', UnitCode='mg'); insert u;
  r5out.add('R5|uom|OK Type=' + [SELECT Type FROM UnitOfMeasure WHERE Id=:u.Id].Type);
} catch (Exception e) { r5out.add('R5|uom|EXC|' + e.getMessage()); }

    System.assert(false, String.join(r5out, ';'));
  }
}
