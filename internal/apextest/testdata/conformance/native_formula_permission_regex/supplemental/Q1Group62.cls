@IsTest private class Q1Group62 {
 @IsTest static void capture() {
  PermissionSetGroup g = [SELECT Id, Status FROM PermissionSetGroup WHERE DeveloperName='Probe_Group' LIMIT 1];
  Test.calculatePermissionSetGroup(g.Id);
  Profile p = [SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u = new User(Alias='q1group', Email='q1group@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=p.Id, Username='q1group'+DateTime.now().getTime()+'@example.invalid');
  insert u;
  PermissionSet rowAccess = [SELECT Id FROM PermissionSet WHERE Name='Probe_Row_Access' LIMIT 1];
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=rowAccess.Id);
  List<String> beforeRow;
  System.runAs(u) { beforeRow = NativeFormulaProbe62.current(); }
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetGroupId=g.Id);
  List<String> afterRow;
  System.runAs(u) { afterRow = NativeFormulaProbe62.current(); }
  System.assert(false,'Q1GROUP|before='+String.join(beforeRow,';')+'|after='+String.join(afterRow,';'));
 }
}
