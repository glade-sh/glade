@IsTest private class AccessProbeTest {
 @IsTest static void observe() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u=new User(ProfileId=p.Id,Username='probe.'+System.now().getTime()+'@example.invalid',Email='probe@example.invalid',Alias='probe',LastName='Probe',TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u;
  String observations='admin67:'+AccessProbe67.run();
  System.runAs(u) { observations+=';user:create='+ProbeSignal__e.SObjectType.getDescribe().isCreateable()+',read='+ProbeSignal__e.SObjectType.getDescribe().isAccessible()+';v62:'+AccessProbe62.run()+';v66:'+AccessProbe66.run()+';v67:'+AccessProbe67.run()+';v68:'+AccessProbe68.run(); }
  insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ProbeReadOnly'].Id);
  System.runAs(u) { observations+=';readOnly:create='+ProbeSignal__e.SObjectType.getDescribe().isCreateable()+',read='+ProbeSignal__e.SObjectType.getDescribe().isAccessible()+';v67:'+AccessProbe67.run(); }
  insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ProbePublish'].Id);
  System.runAs(u) { observations+=';grant:create='+ProbeSignal__e.SObjectType.getDescribe().isCreateable()+',read='+ProbeSignal__e.SObjectType.getDescribe().isAccessible()+';v67:'+AccessProbe67.run(); }
  System.assertEquals('capture', observations);
 }
}
