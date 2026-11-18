@IsTest private class PsrFinishLockTest {
 @TestSetup static void setup() { insert as system new PsrObj__c(Name='lock'); }
 @IsTest static void lockAfterAssignment() {
  Id obj=[SELECT Id FROM PsrObj__c WHERE Name='lock' LIMIT 1].Id;
  Id ps=PsrUtil.permSetId();
  String result=''; String later='';
  insert as system new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(), PermissionSetId=ps);
  try { Approval.LockResult x=Approval.lock(obj, false); result='success:'+x.isSuccess(); if (!x.isSuccess()) result+=':'+x.getErrors()[0].getStatusCode(); } catch(Exception e) { result='ex:'+e.getTypeName()+':'+e.getMessage(); }
  System.runAs(new User(Id=UserInfo.getUserId())) { try { later='locked:'+Approval.isLocked(obj)+';perm:'+FeatureManagement.checkPermission('PsrPerm'); } catch(Exception e) { later='ex:'+e.getTypeName(); } }
  System.assert(false,'after='+result+';later='+later);
 }

 @IsTest static void lockInLaterRunAs() {
  Id obj=[SELECT Id FROM PsrObj__c WHERE Name='lock' LIMIT 1].Id;
  Id ps=PsrUtil.permSetId();
  insert as system new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=ps);
  String result=''; Boolean perm=false; Boolean locked=false;
  System.runAs(new User(Id=UserInfo.getUserId())) {
   perm=FeatureManagement.checkPermission('PsrPerm');
   try { Approval.LockResult x=Approval.lock(obj,false); result='success:'+x.isSuccess(); if (!x.isSuccess()) result+=':'+x.getErrors()[0].getStatusCode(); locked=Approval.isLocked(obj); } catch(Exception e) {result='ex:'+e.getTypeName()+':'+e.getMessage();}
  }
  System.assert(false,'laterPerm='+perm+';lock='+result+';locked='+locked);
 }
}
