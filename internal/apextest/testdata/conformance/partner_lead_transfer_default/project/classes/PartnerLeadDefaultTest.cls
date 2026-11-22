@IsTest private class PartnerLeadDefaultTest {
 @IsTest static void observe() {
  String result = 'unset';
  try {
   PclSupport.setup();
   Contact c = [SELECT AccountId FROM Contact WHERE LastName = 'PclCon' LIMIT 1];
   Account a = [SELECT Id, IsPartner FROM Account WHERE Id = :c.AccountId];
   System.runAs(PclSupport.owner) { a.IsPartner = true; update a; }
   User u = PclSupport.portal('Partner Community User');
   PclLeadGrant.grant(u);
   Lead l = new Lead(LastName='Synthetic', Company='Synthetic'); System.runAs(PclSupport.owner) { insert l; }
   UserRecordAccess access = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:l.Id];
   result = 'access=' + access.HasReadAccess + ',' + access.HasEditAccess;
   System.runAs(u) { result += '|describe=' + Lead.SObjectType.getDescribe().isAccessible() + '|count=' + [SELECT COUNT() FROM Lead WHERE Id=:l.Id]; }
  } catch (Exception e) { result = 'exception=' + e.getTypeName() + ':' + e.getMessage(); }
  System.assert(false, result);
 }
}
