@IsTest private class FlowProfileContext62Proof {
 @IsTest static void observe() {
  Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User limited=new User(Alias='flowprof',Email='flowprofile@example.invalid',EmailEncodingKey='UTF-8',LastName='FlowProfile',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=profile.Id,Username='flowprofile.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert limited;
  PermissionSet grant=[SELECT Id FROM PermissionSet WHERE Name='Flow_Profile_Access' LIMIT 1];
  insert new PermissionSetAssignment(AssigneeId=limited.Id,PermissionSetId=grant.Id);
  Map<String,Object> row=new Map<String,Object>();
  System.runAs(new User(Id=UserInfo.getUserId())) { row.put('administrator',FlowProfileContext62.current()); }
  System.runAs(limited) { row.put('standard',FlowProfileContext62.current()); }
  System.runAs(new User(Id=UserInfo.getUserId())) { row.put('restored',FlowProfileContext62.current()); }
  System.assert(false,'FLOW_PROFILE|'+JSON.serialize(row));
 }
}
