@IsTest(SeeAllData=true) private class ShareCommitTest {
 @IsTest static void observe() {
  User u=[SELECT Id FROM User WHERE Alias='shr' LIMIT 1];
  ShareProbe__c r=[SELECT Id FROM ShareProbe__c WHERE Name='commit-control' LIMIT 1];
  List<UserRecordAccess> a=[SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:r.Id];
  Integer visible; System.runAs(u) { visible=ShareProbeReader.countRows(r.Id); }
  System.assert(false, 'row='+a.size()+':read='+(a.isEmpty() ? 'missing' : String.valueOf(a[0].HasReadAccess))+':query='+visible);
 }
}
