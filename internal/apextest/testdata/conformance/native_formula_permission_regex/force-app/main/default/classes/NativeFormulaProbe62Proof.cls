@IsTest private class NativeFormulaProbe62Proof {
 static User makeUser(String alias, String permissionSet) {
  Profile profile = [SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u = new User(Alias=alias, Email=alias+'@example.invalid', EmailEncodingKey='UTF-8', LastName=alias, LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=profile.Id, Username=alias+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert u;
  PermissionSet grant = [SELECT Id FROM PermissionSet WHERE Name=:permissionSet LIMIT 1];
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=grant.Id);
  return u;
 }
 static void finish(List<String> row) {
  System.assert(false, 'NATIVE_FORMULA|'+String.join(row, ';'));
 }
 @IsTest static void administrator() {
  List<String> row;
  System.runAs(new User(Id=UserInfo.getUserId())) { row = NativeFormulaProbe62.current(); }
  finish(row);
 }
 @IsTest static void rowOnly() {
  User u = makeUser('prbrow', 'Probe_Row_Access');
  List<String> row;
  System.runAs(u) { row = NativeFormulaProbe62.current(); }
  finish(row);
 }
 @IsTest static void bypass() {
  User u = makeUser('prbbyp', 'Probe_Bypass_Access');
  List<String> row;
  System.runAs(u) { row = NativeFormulaProbe62.current(); }
  finish(row);
 }
}
