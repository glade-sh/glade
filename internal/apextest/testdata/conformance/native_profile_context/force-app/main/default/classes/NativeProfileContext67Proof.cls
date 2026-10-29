@IsTest private class NativeProfileContext67Proof {
 @IsTest static void observe() {
  Profile profile = [SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User limited = new User(Alias='ctxprobe', Email='context@example.invalid', EmailEncodingKey='UTF-8', LastName='Context', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=profile.Id, Username='context.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert limited;
  PermissionSet grant = [SELECT Id FROM PermissionSet WHERE Name='Native_Context_Row_Access' LIMIT 1];
  insert new PermissionSetAssignment(AssigneeId=limited.Id, PermissionSetId=grant.Id);
  Map<String,Object> row = new Map<String,Object>();
  System.runAs(new User(Id=UserInfo.getUserId())) {
   row.put('administrator', NativeProfileContext67.current());
   row.put('sameUserRestricted', NativeProfileContext67.save('restricted'));
  }
  System.runAs(limited) { row.put('standard', NativeProfileContext67.current()); }
  System.assert(false, 'NATIVE_CONTEXT|'+JSON.serialize(row));
 }
}
