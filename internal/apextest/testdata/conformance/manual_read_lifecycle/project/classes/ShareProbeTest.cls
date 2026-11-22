@IsTest private class ShareProbeTest {
 @IsTest static void observe() {
  Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
  User u = new User(Alias='shread', Email='reader@example.invalid', EmailEncodingKey='UTF-8', LastName='Reader', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=p.Id, UserName='reader.'+System.now().getTime()+'@example.invalid');
  insert u;
  PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name='SyntheticReaderPermission' LIMIT 1];
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=ps.Id);
  List<String> obs = new List<String>();
  System.runAs(new User(Id=UserInfo.getUserId())) {
   ShareProbe__c r = new ShareProbe__c(Name='Synthetic'); insert r;
   obs.add(checkpoint('denied',u,r));
   ShareProbe__Share s = new ShareProbe__Share(ParentId=r.Id, UserOrGroupId=u.Id, AccessLevel='Read', RowCause='Manual'); insert s;
   obs.add(checkpoint('granted',u,r));
   delete s;
   obs.add(checkpoint('revoked',u,r));
  }
  System.assert(false, String.join(obs,'|'));
 }
 static String checkpoint(String name,User u,ShareProbe__c r) {
  List<UserRecordAccess> a = [SELECT RecordId,HasReadAccess FROM UserRecordAccess WHERE UserId=:u.Id AND RecordId=:r.Id];
  Integer visible; System.runAs(u) { visible=ShareProbeReader.countRows(r.Id); }
  return name+':row='+a.size()+':read='+(a.isEmpty() ? 'missing' : String.valueOf(a[0].HasReadAccess))+':query='+visible;
 }
}
